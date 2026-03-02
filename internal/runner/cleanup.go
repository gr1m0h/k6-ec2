package runner

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/k6-distributed/k6-aws-common/pkg/types"
)

// CleanupInstances terminates EC2 instances.
// If params.Force is true, cleanup proceeds regardless of the cleanup policy.
func (r *Runner) CleanupInstances(ctx context.Context, params *CleanupParams) error {
	if params.Force {
		return r.terminateInstances(ctx, params.InstanceIDs)
	}

	r.setPhase(types.PhaseFinishing)
	return r.cleanup(ctx)
}

func (r *Runner) cleanup(ctx context.Context) error {
	policy := r.spec.Spec.Cleanup.Policy
	shouldCleanup := policy == "always" ||
		(policy == "on-success" && r.phase == types.PhaseCompleted)

	if !shouldCleanup {
		r.logger.Info("skipping cleanup", "policy", policy)
		return nil
	}

	ids := r.instanceIDs()
	if len(ids) == 0 {
		return nil
	}

	return r.terminateInstances(ctx, ids)
}

func (r *Runner) terminateInstances(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	r.logger.Info("terminating instances", "count", len(ids))
	_, err := r.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instances: %w", err)
	}

	r.logger.Info("instances terminated")
	return nil
}
