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

func newPrepareCmd() *cobra.Command {
	var (
		configFile string
		stateFile  string
	)

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare script and resolve AMI for a test run",
		Long: `Uploads the k6 test script to S3 and resolves the AMI.
Writes a state file that can be consumed by subsequent pipeline commands.`,
		Example: `  k6-ec2 prepare -f testrun.yaml
  k6-ec2 prepare -f testrun.yaml --state /tmp/state.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			spec, err := ec2config.LoadForCommand(configFile, ec2config.CommandPrepare, buildOverrides(cmd))
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

			state := &runner.PipelineState{
				TestName:  spec.Name,
				Region:    r.Region(),
				ScriptS3:  prep.ScriptS3,
				AMI:       prep.AMI,
				LogGroup:  r.LogGroup(),
				Phase:     "prepared",
				CreatedAt: time.Now(),
			}

			if err := runner.SaveState(stateFile, state); err != nil {
				return err
			}

			fmt.Printf("✓ Prepare complete\n")
			fmt.Printf("  AMI:      %s\n", prep.AMI)
			fmt.Printf("  Script:   s3://%s/%s\n", prep.ScriptS3.Bucket, prep.ScriptS3.Key)
			fmt.Printf("  State:    %s\n", stateFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to test run config")
	cmd.Flags().StringVar(&stateFile, "state", runner.DefaultStateFile, "Path to state file")
	cmd.Flags().String("region", "", "Override execution.region")
	return cmd
}
