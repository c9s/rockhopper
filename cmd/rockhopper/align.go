package main

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	rootCmd.AddCommand(AlignCmd)
}

var AlignCmd = &cobra.Command{
	Use:   "align",
	Short: "align migration version",

	Args: cobra.ExactArgs(2),

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         align,
}

func align(cmd *cobra.Command, args []string) error {
	packageName := args[0]
	versionStr := args[1]
	versionID, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := rockhopper.OpenWithConfig(config)
	if err != nil {
		return err
	}

	defer db.Close()

	if err := db.Touch(ctx); err != nil {
		return err
	}

	loader := rockhopper.NewSqlMigrationLoader(config)

	migrations, err := loader.Load(config.MigrationsDirs...)
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		log.Infof("no migrations found")
		return nil
	}

	migrations = migrations.FilterPackage([]string{packageName})
	migrations = migrations.SortAndConnect()
	return rockhopper.Align(ctx, db, versionID, migrations)
}
