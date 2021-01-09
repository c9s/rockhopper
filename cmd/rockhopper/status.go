package main

import "github.com/spf13/cobra"

func init() {
	RootCmd.AddCommand(StatusCmd)
}

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show migration status",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         status,
}

func status(cmd *cobra.Command, args []string) error {
	return nil
}
