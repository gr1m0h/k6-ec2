package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		Short: "Resolve AMI and launch EC2 instances for a test run",
		Long: `Resolves the AMI (auto-detects latest Amazon Linux 2023 if not specified)
and launches EC2 instances. Writes a state file for subsequent pipeline commands.`,
		Example: `  k6-ec2 launch -f testrun.yaml
  k6-ec2 launch -f testrun.yaml --state /tmp/state.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			spec, err := ec2config.LoadForCommand(configFile, ec2config.CommandLaunch, buildOverrides(cmd))
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
			go func() { <-sigCh; cancel() }()

			prep, err := r.Prepare(ctx)
			if err != nil {
				return err
			}

			lr, err := r.Launch(ctx, &runner.LaunchParams{
				AMI: prep.AMI,
			})
			if err != nil {
				return err
			}

			state := &runner.PipelineState{
				TestName:      spec.Name,
				Region:        r.Region(),
				AMI:           prep.AMI,
				Instances:     lr.Instances,
				SpotCount:     lr.SpotCount,
				FallbackCount: lr.FallbackCount,
				LogGroup:      r.LogGroup(),
				Phase:         "launched",
				CreatedAt:     time.Now(),
			}

			if err := runner.SaveState(stateFile, state); err != nil {
				return err
			}

			fmt.Printf("✓ Launch complete\n")
			fmt.Printf("  AMI:       %s\n", prep.AMI)
			fmt.Printf("  Instances: %d\n", len(lr.Instances))
			fmt.Printf("  Spot:      %d\n", lr.SpotCount)
			fmt.Printf("  Fallback:  %d\n", lr.FallbackCount)
			for _, inst := range lr.Instances {
				fmt.Printf("  - %s (runner-%d)\n", inst.InstanceID, inst.RunnerID)
			}
			fmt.Printf("  State:     %s\n", stateFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to test run config")
	cmd.Flags().StringVar(&stateFile, "state", runner.DefaultStateFile, "Path to state file")
	cmd.Flags().Int32P("parallelism", "p", 0, "Override runner.parallelism")
	cmd.Flags().String("instance-type", "", "Override runner.instanceType")
	cmd.Flags().String("region", "", "Override execution.region")
	return cmd
}
