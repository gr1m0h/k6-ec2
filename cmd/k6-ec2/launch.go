package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	ec2config "github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/internal/runner"
	"github.com/spf13/cobra"
)

func newLaunchCmd() *cobra.Command {
	var (
		configFile string
		stateFile  string
	)

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch EC2 instances for a test run",
		Long: `Launches EC2 instances using the AMI and script location from the state file.
Requires a prior 'prepare' step.`,
		Example: `  k6-ec2 launch -f testrun.yaml
  k6-ec2 launch -f testrun.yaml --state /tmp/state.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			spec, err := ec2config.Load(configFile)
			if err != nil {
				return err
			}

			state, err := runner.LoadState(stateFile)
			if err != nil {
				return fmt.Errorf("state file required (run 'prepare' first): %w", err)
			}

			r, err := runner.New(spec, logger)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-sigCh; cancel() }()

			lr, err := r.Launch(ctx, &runner.LaunchParams{
				AMI:      state.AMI,
				ScriptS3: state.ScriptS3,
			})
			if err != nil {
				return err
			}

			state.Instances = lr.Instances
			state.SpotCount = lr.SpotCount
			state.FallbackCount = lr.FallbackCount
			state.Phase = "launched"

			if err := runner.SaveState(stateFile, state); err != nil {
				return err
			}

			fmt.Printf("✓ Launch complete\n")
			fmt.Printf("  Instances: %d\n", len(lr.Instances))
			fmt.Printf("  Spot:      %d\n", lr.SpotCount)
			fmt.Printf("  Fallback:  %d\n", lr.FallbackCount)
			for _, inst := range lr.Instances {
				fmt.Printf("  - %s (runner-%d)\n", inst.InstanceID, inst.RunnerID)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to test run config")
	cmd.Flags().StringVar(&stateFile, "state", runner.DefaultStateFile, "Path to state file")
	return cmd
}
