package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/k6-distributed/k6-aws-common/pkg/output"
	"github.com/k6-distributed/k6-aws-common/pkg/types"
	"github.com/k6-distributed/k6-ec2/internal/config"
)

// Execute runs k6 on the given instances via SSM or waits for UserData completion.
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

	if r.spec.Spec.Execution.IsSSMEnabled() {
		if err := r.waitForSSMReady(ctx); err != nil {
			return nil, fmt.Errorf("SSM readiness wait failed: %w", err)
		}

		if err := r.executeViaSSM(ctx, params); err != nil {
			return nil, fmt.Errorf("SSM execution failed: %w", err)
		}

		if err := r.waitForSSMCompletion(ctx); err != nil {
			return nil, fmt.Errorf("SSM monitoring failed: %w", err)
		}
	} else {
		if err := r.waitForInstanceCompletion(ctx); err != nil {
			return nil, fmt.Errorf("instance monitoring failed: %w", err)
		}
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

	cmd := output.BuildK6Command(&r.spec.Spec.Output, "/tmp/test.js", r.spec.Spec.Runner.Arguments)

	// For external instances, prepend S3 script download
	if params != nil && params.ExternalInstances && params.ScriptS3 != nil {
		downloadCmd := fmt.Sprintf("aws s3 cp s3://%s/%s /tmp/test.js", params.ScriptS3.Bucket, params.ScriptS3.Key)
		cmd = downloadCmd + " && " + cmd
	}

	res, err := r.ssmClient.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  r.instanceIDs(),
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			"commands":         {cmd},
			"executionTimeout": {"3600"},
		},
		Comment: aws.String(fmt.Sprintf("k6-ec2: %s", r.spec.Metadata.Name)),
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

func (r *Runner) waitForInstanceCompletion(ctx context.Context) error {
	r.logger.Info("monitoring instance completion (user-data mode)...")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.refreshInstanceStatus(ctx); err != nil {
				r.logger.Warn("failed to refresh instance status", "error", err)
				continue
			}

			allCompleted := true
			for _, inst := range r.instances {
				if inst.ExitCode == nil {
					allCompleted = false
				}
			}

			if allCompleted {
				r.logger.Info("all instances completed")
				return nil
			}
		}
	}
}

func (r *Runner) refreshInstanceStatus(ctx context.Context) error {
	res, err := r.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: r.instanceIDs(),
	})
	if err != nil {
		return err
	}

	for _, reservation := range res.Reservations {
		for _, inst := range reservation.Instances {
			id := aws.ToString(inst.InstanceId)
			for i := range r.instances {
				if r.instances[i].InstanceID == id {
					r.instances[i].State = string(inst.State.Name)
					if inst.PublicIpAddress != nil {
						r.instances[i].PublicIP = aws.ToString(inst.PublicIpAddress)
					}
					if inst.PrivateIpAddress != nil {
						r.instances[i].PrivateIP = aws.ToString(inst.PrivateIpAddress)
					}
					// Check exit code tag
					for _, tag := range inst.Tags {
						if aws.ToString(tag.Key) == "k6-ec2/exit-code" {
							code := 0
							fmt.Sscanf(aws.ToString(tag.Value), "%d", &code)
							r.instances[i].ExitCode = &code
						}
					}
				}
			}
		}
	}

	return nil
}
