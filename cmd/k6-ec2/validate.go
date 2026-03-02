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
			spec, err := ec2config.Load(configFile)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Configuration is valid\n")
			fmt.Printf("  Name:          %s\n", spec.Metadata.Name)
			fmt.Printf("  Parallelism:   %d\n", spec.Spec.Runner.Parallelism)
			fmt.Printf("  Instance Type: %s\n", spec.Spec.Runner.InstanceType)
			fmt.Printf("  Spot:          %v\n", spec.Spec.Runner.Spot.Enabled)
			fmt.Printf("  SSM:           %v\n", spec.Spec.Execution.IsSSMEnabled())
			fmt.Printf("  Subnets:       %d\n", len(spec.Spec.Execution.Subnets))
			return nil
		},
	}

	cmd.Flags().StringVarP(&configFile, "file", "f", "testrun.yaml", "Path to config")
	return cmd
}
