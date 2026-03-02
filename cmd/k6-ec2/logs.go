package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/k6-distributed/k6-aws-common/pkg/monitor"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var (
		testName string
		follow   bool
		region   string
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View logs from a test run",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(cmd)
			logGroup := fmt.Sprintf("/k6-ec2/%s", testName)

			awsCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
			if err != nil {
				return err
			}

			mon := monitor.NewLogMonitor(
				cloudwatchlogs.NewFromConfig(awsCfg),
				logGroup, logger,
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-sigCh; cancel() }()

			if follow {
				return mon.StreamLogs(ctx, testName, func(e monitor.LogEvent) {
					fmt.Printf("[%s] [%s] %s\n", e.Timestamp.Format("15:04:05"), e.RunnerID, e.Message)
				})
			}

			events, err := mon.GetAllLogs(ctx, testName)
			if err != nil {
				return err
			}
			for _, e := range events {
				fmt.Printf("[%s] [%s] %s\n", e.Timestamp.Format("15:04:05"), e.RunnerID, e.Message)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&testName, "test-name", "", "Test run name")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream logs in real-time")
	cmd.Flags().StringVar(&region, "region", "", "AWS region")
	_ = cmd.MarkFlagRequired("test-name")
	return cmd
}
