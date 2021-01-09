package main

import "github.com/spf13/cobra"

func init() {
	UpCmd.Flags().String("to", "", "up to a specific version")
	UpCmd.Flags().Int("step", 0, "run upgrade by steps")
	RootCmd.AddCommand(UpCmd)
}

var UpCmd = &cobra.Command{
	Use:   "up",
	Short: "upgrade database",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         up,
}

func up(cmd *cobra.Command, args []string) error {
	return nil
}
