package main

import (
	"log/slog"
	"os"

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

Pipeline commands: prepare -> launch -> execute -> cleanup
Or use 'run' or a single-command full lifecycle.

See also: k6-ecs for running on ECS Fargate/EC2 launch type.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level(debug, info, warn, error)")
	root.AddCommand(
		newRunCmd(),
		newPrepareCmd(),
		newLaunchCmd(),
		newExecuteCmd(),
		newValidateCmd(),
		newLogsCmd(),
		newInitCmd(),
		newVersionCmd(),
	)
	return root
}

func newLogger(cmd *cobra.Command) *slog.Logger {
	levelStr := cmd.Root().PersistentFlags().GetString("log-level")
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
	return slog.New(slog.NewTextHandler(os.Stderr, &log.HandlerOptions{Level: level}))
}

const sampleConfig = `apiVersion: k6-ec2.io/v1alpha1
kind: EC2TestRun
metadata:
  name: my-load-test
  labels:
    team: platform
	env: staging
spec:
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
	  k6_BATCH: "20"
	  k6_BATCH_PER_HOST: "5"

  execution:
	subnets:
	  - subnet-xxxxxxxxxx
	securityGroups:
	  - sg-xxxxxxxxxx
	assignPublicIP: true
	region: ap-northeast-1
	timeout: 30m
	ssmEnabled: true
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

  cleanup:
	policy: always
`
