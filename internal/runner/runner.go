package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/pkg/result"
	"github.com/gr1m0h/k6-ec2/pkg/script"
	"github.com/gr1m0h/k6-ec2/pkg/types"
)

// Runner orchestrates a distributed k6 test run on EC2 instances.
type Runner struct {
	spec      *config.Config
	ec2Client *ec2.Client
	ssmClient *ssm.Client
	scripts   *script.Resolver
	logger    *slog.Logger

	phase         types.TestRunPhase
	startTime     *time.Time
	endTime       *time.Time
	instances     []config.InstanceStatus
	commandID     string
	spotCount     int
	fallbackCount int
}

// New creates a new EC2 Runner.
func New(spec *config.Config, logger *slog.Logger) (*Runner, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if spec.Execution.Region != "" {
		opts = append(opts, awsconfig.WithRegion(spec.Execution.Region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Runner{
		spec:      spec,
		ec2Client: ec2.NewFromConfig(cfg),
		ssmClient: ssm.NewFromConfig(cfg),
		scripts:   script.NewResolver(),
		logger:    logger,
		phase:     types.PhaseInitializing,
	}, nil
}

// Run executes the full lifecycle by calling Prepare, Launch, Execute, and cleanup.
func (r *Runner) Run(ctx context.Context) error {
	r.logger.Info("starting EC2 test run",
		"name", r.spec.Name,
		"parallelism", r.spec.Runner.Parallelism,
		"instanceType", r.spec.Runner.InstanceType,
		"spot", r.spec.Runner.Spot.Enabled,
	)

	timeout, _ := time.ParseDuration(r.spec.Execution.Timeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	now := time.Now()
	r.startTime = &now

	// Steps 1-2: Prepare script + resolve AMI
	prep, err := r.Prepare(ctx)
	if err != nil {
		return r.fail(err)
	}

	// Step 3-3b: Launch instances + associate EIPs
	_, err = r.Launch(ctx, &LaunchParams{AMI: prep.AMI})
	if err != nil {
		return r.fail(err)
	}

	// Steps 4-6: Execute (wait for SSM, run, monitor)
	_, err = r.Execute(ctx, &ExecuteParams{InstanceIDs: r.instanceIDs()})
	if err != nil {
		return r.fail(err)
	}

	// Step 7: Cleanup
	r.setPhase(types.PhaseFinishing)
	if err := r.cleanup(ctx); err != nil {
		r.logger.Warn("cleanup failed", "error", err)
	}

	r.setPhase(types.PhaseCompleted)
	end := time.Now()
	r.endTime = &end
	r.logger.Info("test run completed", "duration", end.Sub(now).Round(time.Second))

	return r.EvaluateResults()
}

// Cancel stops the running test.
func (r *Runner) Cancel(ctx context.Context) error {
	r.logger.Info("cancelling test run")
	r.setPhase(types.PhaseCancelled)

	if r.commandID != "" {
		for _, inst := range r.instances {
			_, _ = r.ssmClient.CancelCommand(ctx, &ssm.CancelCommandInput{
				CommandId:   aws.String(r.commandID),
				InstanceIds: []string{inst.InstanceID},
			})
		}
	}

	return r.cleanup(ctx)
}

// Summary returns the result summary.
func (r *Runner) Summary() *result.Summary {
	var results []types.RunnerResult
	for _, inst := range r.instances {
		results = append(results, types.RunnerResult{
			ID:       inst.InstanceID,
			Label:    fmt.Sprintf("runner-%d", inst.RunnerID),
			Status:   inst.State,
			ExitCode: inst.ExitCode,
		})
	}

	var spot *types.SpotInfo
	if r.spec.Runner.Spot.Enabled {
		spot = &types.SpotInfo{
			Used:     r.spotCount > 0,
			Count:    r.spotCount,
			Fallback: r.fallbackCount,
		}
	}

	return result.NewSummary(
		r.spec.Name, "EC2/"+r.spec.Runner.InstanceType,
		string(r.phase), r.spec.Runner.Parallelism,
		r.startTime, r.endTime, results, spot,
	)
}

// LogGroup returns the CloudWatch log group name.
func (r *Runner) LogGroup() string {
	return fmt.Sprintf("/k6-ec2/%s", r.spec.Name)
}

// Region returns the configured region.
func (r *Runner) Region() string {
	return r.spec.Execution.Region
}

// EvaluateResults checks if any instances failed and returns an error if so.
func (r *Runner) EvaluateResults() error {
	var failed []string
	for _, inst := range r.instances {
		if inst.ExitCode != nil && *inst.ExitCode != 0 {
			failed = append(failed, fmt.Sprintf("instance %s exited with code %d", inst.InstanceID, *inst.ExitCode))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d instance(s) failed: %s", len(failed), strings.Join(failed, "; "))
	}
	return nil
}

// --- Internal helpers ---

func (r *Runner) setPhase(phase types.TestRunPhase) {
	r.phase = phase
	r.logger.Info("phase", "phase", phase)
}

func (r *Runner) fail(err error) error {
	r.setPhase(types.PhaseFailed)
	end := time.Now()
	r.endTime = &end
	_ = r.cleanup(context.Background())
	return err
}

func (r *Runner) instanceIDs() []string {
	ids := make([]string, 0, len(r.instances))
	for _, inst := range r.instances {
		ids = append(ids, inst.InstanceID)
	}
	return ids
}
