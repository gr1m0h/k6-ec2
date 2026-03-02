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
	"github.com/spf13/cobra"
)

func newExecuteCmd() *cobra.Command {
	var (
		configFile  string
		stateFile   string
		instanceIDs string
		scriptPath  string
	)

	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute k6 on running EC2 instances via SSM",
		Long: `Executes k6 tests on EC2 instances via SSM SendCommand.
The test script is transferred inline (no S3 required).
Uses instance IDs from the state file, or specify --instance-ids for pre-existing instances.`,
		Example: `  k6-ec2 execute -f testrun.yaml
  k6-ec2 execute -f testrun.yaml --state /tmp/state.json
  k6-ec2 execute -f testrun.yaml --instance-ids i-abc123,i-def456
  k6-ec2 execute -f testrun.yaml --instance-ids i-abc123 --script ./test.js`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			spec, err := ec2config.LoadForCommand(configFile, ec2config.CommandExecute, buildOverrides(cmd))
			if err != nil {
				return err
			}

			// --script flag overrides config script.localFile
			if scriptPath != "" {
				spec.Script.LocalFile = scriptPath
				spec.Script.LocalDir = ""
				spec.Script.Entrypoint = ""
				spec.Script.Inline = ""
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
			} else {
				// Load from state file
				state, err := runner.LoadState(stateFile)
				if err != nil {
					return fmt.Errorf("state file required (run 'launch' first, or use --instance-ids): %w", err)
				}
				params.InstanceIDs = state.InstanceIDs()
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
	cmd.Flags().StringVar(&scriptPath, "script", "", "Path to k6 test script (overrides config script.localFile)")
	cmd.Flags().String("region", "", "Override execution.region")
	cmd.Flags().String("timeout", "", "Override execution.timeout")
	return cmd
}
