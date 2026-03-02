package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a sample test run configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(sampleConfig), 0644); err != nil {
					return err
				}
				fmt.Printf("✓ Sample configuration written to %s\n", outputFile)
				return nil
			}
			fmt.Print(sampleConfig)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	return cmd
}
