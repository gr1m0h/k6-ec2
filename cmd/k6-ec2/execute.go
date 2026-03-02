package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	ec2config "github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/internal/runner"
	"github.com/gr1m0h/k6-ec2/pkg/types"
	"github.com/spf13/cobra"
)

func newExecuteCmd() *cobra.Command {
	var (
		configFile  string
		stateFile   string
		instanceIDs string
		scriptS3URI string
	)

	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute k6 on running EC2 instances",
		Long: `Executes k6 tests on EC2 instances via SSM or waits for UserData completion.
Uses instance IDs from the state file, or specify --instance-ids for pre-existing instances.`,
		Example: `  k6-ec2 execute -f testrun.yaml
  k6-ec2 execute -f testrun.yaml --state /tmp/state.json
  k6-ec2 execute -f testrun.yaml --instance-ids i-abc123,i-def456 --script-s3 s3://bucket/test.js`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			spec, err := ec2config.LoadForCommand(configFile, ec2config.CommandExecute, buildOverrides(cmd))
			if err != nil {
				return err
			}

			r, err := runner.New(spec, logger)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				logger.Info("received shutdown signal, cancelling...")
				_ = r.Cancel(ctx)
				cancel()
			}()

			params := &runner.ExecuteParams{}

			if instanceIDs != "" {
				// External instances mode
				params.InstanceIDs = strings.Split(instanceIDs, ",")
				params.ExternalInstances = true

				if scriptS3URI != "" {
					loc, err := parseS3URI(scriptS3URI)
					if err != nil {
						return err
					}
					params.ScriptS3 = loc
				}
			} else {
				// Load from state file
				state, err := runner.LoadState(stateFile)
				if err != nil {
					return fmt.Errorf("state file required (run 'launch' first, or use --instance-ids): %w", err)
				}
				params.InstanceIDs = state.InstanceIDs()
				params.ScriptS3 = state.ScriptS3
			}

			result, err := r.Execute(ctx, params)
			if err != nil {
				return err
			}

			// Update state file if it was used
			if instanceIDs == "" {
				state, loadErr := runner.LoadState(stateFile)
				if loadErr == nil {
					state.Instances = result.Instances
					state.Phase = "executed"
					_ = runner.SaveState(stateFile, state)
				}
			}

			evalErr := r.EvaluateResults()

			summary := r.Summary()
			fmt.Print(summary.FormatText())

			return evalErr
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to test run config")
	cmd.Flags().StringVar(&stateFile, "state", runner.DefaultStateFile, "Path to state file")
	cmd.Flags().StringVar(&instanceIDs, "instance-ids", "", "Comma-separated EC2 instance IDs (for pre-existing instances)")
	cmd.Flags().StringVar(&scriptS3URI, "script-s3", "", "S3 URI of the test script (e.g., s3://bucket/test.js)")
	cmd.Flags().String("region", "", "Override execution.region")
	cmd.Flags().String("timeout", "", "Override execution.timeout")
	return cmd
}

// parseS3URI parses an S3 URI like "s3://bucket/key" into an S3Location.
func parseS3URI(uri string) (*types.S3Location, error) {
	if !strings.HasPrefix(uri, "s3://") {
		return nil, fmt.Errorf("invalid S3 URI %q: must start with s3://", uri)
	}
	rest := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid S3 URI %q: expected s3://bucket/key", uri)
	}
	return &types.S3Location{
		Bucket: parts[0],
		Key:    parts[1],
	}, nil
}
