package runner

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/pkg/output"
	"github.com/gr1m0h/k6-ec2/pkg/script"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// Execute runs k6 on the given instances via SSM SendCommand.
// If params is provided with InstanceIDs, those are used; otherwise the runner's
// internally tracked instances are used.
func (r *Runner) Execute(ctx context.Context, params *ExecuteParams) (*ExecuteResult, error) {
	r.setPhase(types.PhaseRunning)

	// If external instance IDs are provided, populate r.instances
	if params != nil && len(params.InstanceIDs) > 0 && len(r.instances) == 0 {
		for i, id := range params.InstanceIDs {
			r.instances = append(r.instances, config.InstanceStatus{
				InstanceID: id,
				RunnerID:   int32(i),
				State:      "pending",
			})
		}
	}

	// Resolve script payload if not already provided
	if params.ScriptPayload == nil {
		payload, err := r.scripts.Resolve(&r.spec.Script)
		if err != nil {
			return nil, fmt.Errorf("script resolution failed: %w", err)
		}
		params.ScriptPayload = payload
	}

	if err := r.waitForSSMReady(ctx); err != nil {
		return nil, fmt.Errorf("SSM readiness wait failed: %w", err)
	}

	if err := r.executeViaSSM(ctx, params); err != nil {
		return nil, fmt.Errorf("SSM execution failed: %w", err)
	}

	if err := r.waitForSSMCompletion(ctx); err != nil {
		return nil, fmt.Errorf("SSM monitoring failed: %w", err)
	}

	return &ExecuteResult{
		Instances: r.instances,
		CommandID: r.commandID,
	}, nil
}

func (r *Runner) waitForSSMReady(ctx context.Context) error {
	r.logger.Info("waiting for SSM agent readiness...")

	instanceIDs := r.instanceIDs()
	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for SSM agent on instances")
		case <-ticker.C:
			res, err := r.ssmClient.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
				Filters: []ssmtypes.InstanceInformationStringFilter{
					{
						Key:    aws.String("InstanceIds"),
						Values: instanceIDs,
					},
				},
			})
			if err != nil {
				r.logger.Debug("SSM describe failed", "error", err)
				continue
			}

			online := 0
			for _, info := range res.InstanceInformationList {
				if info.PingStatus == ssmtypes.PingStatusOnline {
					online++
				}
			}

			r.logger.Info("SSM agent status", "online", online, "total", len(instanceIDs))
			if online == len(instanceIDs) {
				return nil
			}
		}
	}
}

func (r *Runner) executeViaSSM(ctx context.Context, params *ExecuteParams) error {
	r.logger.Info("executing k6 via SSM Run Command")

	payload := params.ScriptPayload
	encoded := base64.StdEncoding.EncodeToString(payload.Content)

	// Build script deployment command
	var deployCmd string
	if payload.IsArchive {
		deployCmd = fmt.Sprintf("mkdir -p %s && echo '%s' | base64 -d | tar xzf - -C %s",
			script.ArchiveBaseDir, encoded, script.ArchiveBaseDir)
	} else {
		deployCmd = fmt.Sprintf("echo '%s' | base64 -d | gunzip > %s",
			encoded, payload.Entrypoint)
	}

	k6Cmd := output.BuildK6Command(&r.spec.Output, payload.Entrypoint, r.spec.Runner.Arguments)
	cmd := deployCmd + " && " + k6Cmd

	res, err := r.ssmClient.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  r.instanceIDs(),
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			"commands":         {cmd},
			"executionTimeout": {"3600"},
		},
		Comment: aws.String(fmt.Sprintf("k6-ec2: %s", r.spec.Name)),
		CloudWatchOutputConfig: &ssmtypes.CloudWatchOutputConfig{
			CloudWatchLogGroupName:  aws.String(r.LogGroup()),
			CloudWatchOutputEnabled: true,
		},
	})
	if err != nil {
		return fmt.Errorf("SSM SendCommand failed: %w", err)
	}

	r.commandID = aws.ToString(res.Command.CommandId)
	r.logger.Info("SSM command sent", "commandId", r.commandID)
	return nil
}

func (r *Runner) waitForSSMCompletion(ctx context.Context) error {
	r.logger.Info("monitoring SSM command execution...")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			allDone := true
			for i, inst := range r.instances {
				res, err := r.ssmClient.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
					CommandId:  aws.String(r.commandID),
					InstanceId: aws.String(inst.InstanceID),
				})
				if err != nil {
					r.logger.Debug("GetCommandInvocation failed", "instanceId", inst.InstanceID, "error", err)
					allDone = false
					continue
				}

				status := string(res.Status)
				r.instances[i].State = status

				switch res.Status {
				case ssmtypes.CommandInvocationStatusSuccess:
					code := 0
					r.instances[i].ExitCode = &code
				case ssmtypes.CommandInvocationStatusFailed, ssmtypes.CommandInvocationStatusTimedOut, ssmtypes.CommandInvocationStatusCancelled:
					code := 1
					if res.ResponseCode != 0 {
						c := int(res.ResponseCode)
						code = c
					}
					r.instances[i].ExitCode = &code
				default:
					allDone = false
				}
			}

			if allDone {
				r.logger.Info("all SSM commands completed")
				return nil
			}
		}
	}
}
