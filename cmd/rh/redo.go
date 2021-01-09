package main

import "github.com/spf13/cobra"

func init() {
	RedoCmd.Flags().String("totp-key-url", "", "time-based one-time password key URL, if defined, it will be used for restoring the otp key")
	RootCmd.AddCommand(RedoCmd)
}

var RedoCmd = &cobra.Command{
	Use:   "redo",
	Short: "redo migration",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         redo,
}

func redo(cmd *cobra.Command, args []string) error {
	return nil
}

