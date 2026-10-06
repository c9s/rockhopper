package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These variables are injected at build time via -ldflags "-X main.Version=...".
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the rockhopper version",

	// version must work without a config file
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return nil
	},

	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Printf("rockhopper %s (commit %s, built %s)\n", Version, Commit, BuildTime)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
