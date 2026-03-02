package main

import (
	"fmt"

	ec2config "github.com/gr1m0h/k6-ec2/internal/config"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a test run configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := ec2config.LoadForCommand(configFile, ec2config.CommandValidate, nil)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Configuration is valid\n")
			fmt.Printf("  Name:          %s\n", spec.Name)
			fmt.Printf("  Parallelism:   %d\n", spec.Runner.Parallelism)
			fmt.Printf("  Instance Type: %s\n", spec.Runner.InstanceType)
			fmt.Printf("  Spot:          %v\n", spec.Runner.Spot.Enabled)
			fmt.Printf("  SSM:           %v\n", spec.Execution.IsSSMEnabled())
			fmt.Printf("  Subnets:       %d\n", len(spec.Execution.Subnets))
			return nil
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to config")
	return cmd
}
