package main

import "github.com/spf13/cobra"

func init() {
	DownCmd.Flags().String("to", "", "downgrade to a specific version")
	DownCmd.Flags().Int("steps", 0, "downgrade by steps")
	RootCmd.AddCommand(DownCmd)
}

var DownCmd = &cobra.Command{
	Use:   "down",
	Short: "downgrade database",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         down,
}

func down(cmd *cobra.Command, args []string) error {
	return nil
}


