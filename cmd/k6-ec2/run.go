package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	ec2config "github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/gr1m0h/k6-ec2/internal/runner"
	"github.com/gr1m0h/k6-ec2/pkg/monitor"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		configFile   string
		noLogs       bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a distributed k6 test run on EC2",
		Example: `  k6-ec2 run -f testrun.yaml
  k6-ec2 run -f testrun.yaml --no-logs --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)

			spec, err := ec2config.LoadForCommand(configFile, ec2config.CommandRun, buildOverrides(cmd))
			if err != nil {
				return err
			}
			logger.Info("loaded configuration",
				"name", spec.Name,
				"parallelism", spec.Runner.Parallelism,
				"instanceType", spec.Runner.InstanceType,
				"spot", spec.Runner.Spot.Enabled,
			)

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

			if !noLogs && spec.Execution.IsSSMEnabled() {
				awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(r.Region()))
				if err == nil {
					mon := monitor.NewLogMonitor(
						cloudwatchlogs.NewFromConfig(awsCfg),
						r.LogGroup(), logger,
					)
					logCtx, logCancel := context.WithCancel(ctx)
					defer logCancel()
					go func() {
						_ = mon.StreamLogs(logCtx, spec.Name, func(e monitor.LogEvent) {
							fmt.Printf("[%s] [%s] %s\n", e.Timestamp.Format("15:04:05"), e.RunnerID, e.Message)
						})
					}()
				} else {
					logger.Warn("log streaming unavailable", "error", err)
				}
			}

			runErr := r.Run(ctx)

			summary := r.Summary()
			switch outputFormat {
			case "json":
				if s, err := summary.FormatJSON(); err == nil {
					fmt.Println(s)
				}
			default:
				fmt.Print(summary.FormatText())
			}

			return runErr
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to test run config")
	cmd.Flags().BoolVar(&noLogs, "no-logs", false, "Disable real-time log streaming")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")
	cmd.Flags().Int32P("parallelism", "p", 0, "Override runner.parallelism")
	cmd.Flags().String("instance-type", "", "Override runner.instanceType")
	cmd.Flags().String("region", "", "Override execution.region")
	cmd.Flags().String("timeout", "", "Override execution.timeout")
	cmd.Flags().String("cleanup", "", "Override cleanup policy (always, on-success, never)")
	return cmd
}
