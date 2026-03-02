package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	ec2config "github.com/k6-distributed/k6-ec2/internal/config"
	"github.com/k6-distributed/k6-ec2/internal/runner"
	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	var (
		configFile  string
		stateFile   string
		instanceIDs string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Terminate EC2 instances from a test run",
		Long: `Terminates EC2 instances created by a test run.
Uses instance IDs from the state file, or specify --instance-ids for pre-existing instances.
With --force, instances are terminated regarless of the cleanup policy.`,
		Example: `  k6-ec2 cleanup
  k6-ec2 cleanup --state /tmp/state.json
  k6-ec2 cleanup --instance-ids i-abc123,i-def456 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			params := &runner.CleanupParams{
				Force: force,
			}

			if instanceIDs != "" {
				// External instances mode — no config file needed
				params.InstanceIDs = strings.Split(instanceIDs, ",")
			} else {
				// Load from state file
				state, err := runner.LoadState(stateFile)
				if err != nil {
					return fmt.Errorf("state file required (run 'launch' first, or use --instance-ids): %w", err)
				}
				params.InstanceIDs = state.InstanceIDs()
			}

			if len(params.InstanceIDs) == 0 {
				logger.Info("no instances to clean up")
				return nil
			}

			// Load config if provided (needed for Runner with cleanup policy)
			var r *runner.Runner
			if configFile != "" {
				spec, err := ec2config.Load(configFile)
				if err != nil {
					return err
				}
				r, err = runner.New(spec, logger)
				if err != nil {
					return err
				}
			} else if !force {
				return fmt.Errorf("--file or --force is required for cleanup")
			} else {
				// Force mode without config: create a minimal runner
				spec, err := ec2config.Parse([]byte(minimalConfig))
				if err != nil {
					return err
				}
				r, err = runner.New(spec, logger)
				if err != nil {
					return err
				}
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-sigCh; cancel() }()

			if err := r.CleanupInstances(ctx, params); err != nil {
				return err
			}

			// Update state file if it was used
			if instanceIDs == "" {
				state, loadErr := runner.LoadState(stateFile)
				if loadErr == nil {
					state.Phase = "cleaned"
					_ = runner.SaveState(stateFile, state)
				}
			}

			fmt.Printf("✓ Cleanup complete (%d instances terminated)\n", len(params.InstanceIDs))
			return nil
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "", "Path to test run config (optional with --force)")
	cmd.Flags().StringVar(&stateFile, "state", runner.DefaultStateFile, "Path to state file")
	cmd.Flags().StringVar(&instanceIDs, "instance-ids", "", "Comma-separated EC2 instance IDs (for pre-existing instances)")
	cmd.Flags().BoolVar(&force, "force", false, "Force termination regardless of cleanup policy")
	return cmd
}

// minimalConfig is used when --force is specified without a config file.
const minimalConfig = `apiVersion: k6-ec2.io/v1alpha1
kind: EC2TestRun
metadata:
  name: cleanup
spec:
  script:
    localFile: /dev/null
  runner:
    instanceType: t3.micro
    parallelism: 1
  execution:
    subnets:
      - subnet-placeholder
    securityGroups:
      - sg-placeholder
    region: us-east-1
  cleanup:
    policy: always
`
