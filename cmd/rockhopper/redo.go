package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	RedoCmd.Flags().String("totp-key-url", "", "time-based one-time password key URL, if defined, it will be used for restoring the otp key")
	rootCmd.AddCommand(RedoCmd)
}

var RedoCmd = &cobra.Command{
	Use:   "redo",
	Short: "redo migration",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         redo,
}

func redo(cmd *cobra.Command, args []string) error {
	db, err := rockhopper.OpenWithConfig(config)
	if err != nil {
		return err
	}

	defer db.Close()

	loader := &rockhopper.SqlMigrationLoader{}
	migrations, err := loader.Load(config.MigrationsDir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	currentVersion, err := db.CurrentVersion(ctx, rockhopper.defaultPackageName)
	if err != nil {
		return err
	}

	return rockhopper.Redo(ctx, db, currentVersion, migrations)
}
