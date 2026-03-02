package runner

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/pkg/output"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// Launch starts EC2 instances using the given parameters.
// It returns a LaunchResult with instance details.
func (r *Runner) Launch(ctx context.Context, params *LaunchParams) (*LaunchResult, error) {
	r.scriptS3 = params.ScriptS3
	r.setPhase(types.PhaseStarted)

	if err := r.launchInstances(ctx, params.AMI); err != nil {
		return nil, fmt.Errorf("instance launch failed: %w", err)
	}
	r.logger.Info("instances launched", "count", len(r.instances))

	// Associate EIPs if configured
	if len(r.spec.Execution.EIPAllocationIDs) > 0 {
		if err := r.associateEIPs(ctx); err != nil {
			return nil, fmt.Errorf("EIP association failed: %w", err)
		}
	}

	return &LaunchResult{
		Instances:     r.instances,
		SpotCount:     r.spotCount,
		FallbackCount: r.fallbackCount,
	}, nil
}

func (r *Runner) launchInstances(ctx context.Context, ami string) error {
	subnets := r.spec.Execution.Subnets

	for i := int32(0); i < r.spec.Runner.Parallelism; i++ {
		subnet := subnets[int(i)%len(subnets)]

		userData := r.buildUserData(i)
		encodedUD := base64.StdEncoding.EncodeToString([]byte(userData))

		input := &ec2.RunInstancesInput{
			ImageId:      aws.String(ami),
			InstanceType: ec2types.InstanceType(r.spec.Runner.InstanceType),
			MinCount:     aws.Int32(1),
			MaxCount:     aws.Int32(1),
			UserData:     aws.String(encodedUD),
			NetworkInterfaces: []ec2types.InstanceNetworkInterfaceSpecification{
				{
					DeviceIndex:              aws.Int32(0),
					SubnetId:                 aws.String(subnet),
					Groups:                   r.spec.Execution.SecurityGroups,
					AssociatePublicIpAddress: aws.Bool(r.spec.Execution.AssignPublicIP),
				},
			},
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2types.EbsBlockDevice{
						VolumeSize: aws.Int32(r.spec.Runner.RootVolumeSize),
						VolumeType: ec2types.VolumeTypeGp3,
						Encrypted:  aws.Bool(true),
					},
				},
			},
			MetadataOptions: &ec2types.InstanceMetadataOptionsRequest{
				HttpTokens: ec2types.HttpTokensStateRequired, // IMDSv2
			},
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeInstance,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("k6-ec2-%s-%d", r.spec.Name, i))},
						{Key: aws.String("k6-ec2/test-run"), Value: aws.String(r.spec.Name)},
						{Key: aws.String("k6-ec2/managed-by"), Value: aws.String("k6-ec2")},
						{Key: aws.String("k6-ec2/runner-id"), Value: aws.String(fmt.Sprintf("%d", i))},
					},
				},
			},
		}

		if r.spec.Runner.IAMInstanceProfile != "" {
			input.IamInstanceProfile = &ec2types.IamInstanceProfileSpecification{
				Name: aws.String(r.spec.Runner.IAMInstanceProfile),
			}
		}

		// Try Spot first if enabled
		if r.spec.Runner.Spot.Enabled {
			input.InstanceMarketOptions = &ec2types.InstanceMarketOptionsRequest{
				MarketType: ec2types.MarketTypeSpot,
				SpotOptions: &ec2types.SpotMarketOptions{
					SpotInstanceType: ec2types.SpotInstanceTypeOneTime,
				},
			}
			if r.spec.Runner.Spot.MaxPrice != "" {
				input.InstanceMarketOptions.SpotOptions.MaxPrice = aws.String(r.spec.Runner.Spot.MaxPrice)
			}
		}

		res, err := r.ec2Client.RunInstances(ctx, input)
		if err != nil {
			// Spot fallback
			if r.spec.Runner.Spot.Enabled && r.spec.Runner.Spot.FallbackToOnDemand {
				r.logger.Warn("Spot request failed, falling back to on-demand", "runner", i, "error", err)
				input.InstanceMarketOptions = nil
				res, err = r.ec2Client.RunInstances(ctx, input)
				if err != nil {
					return fmt.Errorf("failed to launch instance %d (on-demand fallback): %w", i, err)
				}
				r.fallbackCount++
			} else {
				return fmt.Errorf("failed to launch instance %d: %w", i, err)
			}
		} else if r.spec.Runner.Spot.Enabled {
			r.spotCount++
		}

		for _, inst := range res.Instances {
			instanceID := aws.ToString(inst.InstanceId)
			spotID := ""
			if inst.SpotInstanceRequestId != nil {
				spotID = aws.ToString(inst.SpotInstanceRequestId)
			}
			r.instances = append(r.instances, config.InstanceStatus{
				InstanceID: instanceID,
				State:      string(inst.State.Name),
				RunnerID:   i,
				SpotID:     spotID,
			})
			r.logger.Info("instance launched", "instanceId", instanceID, "runner", i, "spot", spotID != "")
		}
	}

	return nil
}

func (r *Runner) buildUserData(runnerID int32) string {
	envVars, _ := output.Build(&r.spec.Output)

	var sb strings.Builder
	sb.WriteString("#!/bin/bash\nset -euo pipefail\n\n")
	sb.WriteString("# k6-ec2 auto-generated user data\n")
	sb.WriteString(fmt.Sprintf("export K6_INSTANCE_ID=%d\n", runnerID))
	sb.WriteString(fmt.Sprintf("export K6_PARALLELISM=%d\n", r.spec.Runner.Parallelism))

	// Set env vars
	for k, v := range r.spec.Runner.Env {
		sb.WriteString(fmt.Sprintf("export %s=%q\n", k, v))
	}
	for _, ev := range envVars {
		sb.WriteString(fmt.Sprintf("export %s=%q\n", ev.Key, ev.Value))
	}

	// Install k6
	sb.WriteString("\n# Install k6\n")
	if r.spec.Runner.K6Version == "latest" {
		sb.WriteString("curl -sL https://github.com/grafana/k6/releases/latest/download/k6-linux-amd64.tar.gz | tar xz\n")
	} else {
		sb.WriteString(fmt.Sprintf("curl -sL https://github.com/grafana/k6/releases/download/v%s/k6-v%s-linux-amd64.tar.gz | tar xz\n",
			r.spec.Runner.K6Version, r.spec.Runner.K6Version))
	}
	sb.WriteString("mv k6-*/k6 /usr/local/bin/k6 2>/dev/null || mv k6 /usr/local/bin/k6\n")
	sb.WriteString("chmod +x /usr/local/bin/k6\n")

	// Download script from S3
	sb.WriteString("\n# Download test script\n")
	sb.WriteString(fmt.Sprintf("aws s3 cp s3://%s/%s /tmp/test.js\n", r.scriptS3.Bucket, r.scriptS3.Key))

	// Extra user data
	if r.spec.Runner.UserDataExtra != "" {
		sb.WriteString("\n# Extra user data\n")
		sb.WriteString(r.spec.Runner.UserDataExtra)
		sb.WriteString("\n")
	}

	// Run k6 (only for non-SSM mode)
	if !r.spec.Execution.IsSSMEnabled() {
		sb.WriteString("\n# Run k6\n")
		cmd := output.BuildK6Command(&r.spec.Output, "/tmp/test.js", r.spec.Runner.Arguments)
		sb.WriteString(fmt.Sprintf("EXIT_CODE=0\n%s || EXIT_CODE=$?\n", cmd))

		// Tag instance with exit code
		sb.WriteString("\n# Report status\n")
		sb.WriteString("INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)\n")
		sb.WriteString("REGION=$(curl -s http://169.254.169.254/latest/meta-data/placement/region)\n")
		sb.WriteString("aws ec2 create-tags --region $REGION --resources $INSTANCE_ID --tags Key=k6-ec2/exit-code,Value=$EXIT_CODE Key=k6-ec2/status,Value=completed\n")
	}

	return sb.String()
}

func (r *Runner) associateEIPs(ctx context.Context) error {
	eips := r.spec.Execution.EIPAllocationIDs

	// Wait for all instances to reach "running" state before associating EIPs.
	r.logger.Info("waiting for instances to be running for EIP association...")
	waiter := ec2.NewInstanceRunningWaiter(r.ec2Client)
	if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: r.instanceIDs(),
	}, 5*time.Minute); err != nil {
		return fmt.Errorf("instances did not reach running state: %w", err)
	}

	for i, inst := range r.instances {
		if i >= len(eips) {
			break
		}
		r.logger.Info("associating EIP", "instanceId", inst.InstanceID, "allocationId", eips[i])
		res, err := r.ec2Client.AssociateAddress(ctx, &ec2.AssociateAddressInput{
			InstanceId:   aws.String(inst.InstanceID),
			AllocationId: aws.String(eips[i]),
		})
		if err != nil {
			return fmt.Errorf("failed to associate EIP %s with instance %s: %w", eips[i], inst.InstanceID, err)
		}
		r.logger.Info("EIP associated",
			"instanceId", inst.InstanceID,
			"associationId", aws.ToString(res.AssociationId),
		)
	}

	r.logger.Info("all EIPs associated", "count", len(r.instances))
	return nil
}
