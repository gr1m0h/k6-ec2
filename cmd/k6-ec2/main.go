package main

import (
	"log/slog"
	"os"

	ec2config "github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var logLevel string

	root := &cobra.Command{
		Use:   "k6-ec2",
		Short: "Distributed k6 load testing on AWS EC2",
		Long: `k6-ec2 orchestrates distributed k6 load tests on AWS EC2 instances.

Run k6 directly on EC2 instances with Spot Instance support, SSM-based execution, and automatic lifecycle management, No ECS cluster required.

Pipeline commands: launch -> execute -> cleanup
Or use 'run' for a single-command full lifecycle.

See also: k6-ecs for running on ECS Fargate/EC2 launch type.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level(debug, info, warn, error)")
	root.AddCommand(
		newRunCmd(),
		newLaunchCmd(),
		newExecuteCmd(),
		newCleanupCmd(),
		newValidateCmd(),
		newLogsCmd(),
		newInitCmd(),
		newVersionCmd(),
	)
	return root
}

func newLogger(cmd *cobra.Command) *slog.Logger {
	levelStr, _ := cmd.Root().PersistentFlags().GetString("log-level")
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func buildOverrides(cmd *cobra.Command) *ec2config.Overrides {
	o := &ec2config.Overrides{}
	if cmd.Flags().Changed("parallelism") {
		v, _ := cmd.Flags().GetInt32("parallelism")
		o.Parallelism = &v
	}
	if cmd.Flags().Changed("instance-type") {
		v, _ := cmd.Flags().GetString("instance-type")
		o.InstanceType = &v
	}
	if cmd.Flags().Changed("region") {
		v, _ := cmd.Flags().GetString("region")
		o.Region = &v
	}
	if cmd.Flags().Changed("timeout") {
		v, _ := cmd.Flags().GetString("timeout")
		o.Timeout = &v
	}
	if cmd.Flags().Changed("cleanup") {
		v, _ := cmd.Flags().GetString("cleanup")
		o.Cleanup = &v
	}
	return o
}

const sampleConfig = `name: my-load-test
labels:
  team: platform
  env: staging

script:
  localFile: ./scripts/test.js

runner:
  instanceType: c5.xlarge
  parallelism: 4
  k6Version: latest
  rootVolumeSize: 20
  spot:
    enabled: true
    fallbackToOnDemand: true
  env:
    K6_BATCH: "20"
    K6_BATCH_PER_HOST: "5"

execution:
  subnets:
    - subnet-xxxxxxxxxx
  securityGroups:
    - sg-xxxxxxxxxx
  assignPublicIP: true
  region: ap-northeast-1
  timeout: 30m
  # Uncomment to associate pre-allocated EIPs for WAF IP-based allowlisting.
  # Length must be >= runner.parallelism.
  # eipAllocationIDs:
  #   - eipalloc-xxxxxxxxxxxxxxxxx
  #   - eipalloc-xxxxxxxxxxxxxxxxx
  #   - eipalloc-xxxxxxxxxxxxxxxxx
  #   - eipalloc-xxxxxxxxxxxxxxxxx

output:
  statsd:
    address: "datadog-agent.service.local:8125"
    enabledTags: true
    namespace: "k6."

cleanup: always
`
