package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	CreateCmd.Flags().StringP("type", "t", "sql", "migration type, could be \"go\" or \"sql\"")
	CreateCmd.Flags().StringP("output", "o", "", "output directory")
	rootCmd.AddCommand(CreateCmd)
}

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create",
	Long:  "create",
	Args:  cobra.MinimumNArgs(1),

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         create,
}

func create(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	templateType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}

	outputDir, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	if outputDir == "" && config.MigrationsDir != "" {
		outputDir = config.MigrationsDir
	}

	if outputDir == "" {
		outputDir = "migrations"
	}

	if !dirExists(outputDir) {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
	}

	return rockhopper.CreateWithTemplate(outputDir, nil, args[0], templateType)
}

func dirExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return info.IsDir()
}
