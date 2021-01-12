package main

import (
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	CreateCmd.Flags().StringP("type", "t", "sql", "migration type, could be \"go\" or \"sql\"")
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

	return rockhopper.CreateWithTemplate(config.MigrationsDir, nil, args[0], templateType)
}
