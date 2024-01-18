package main

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	// RedoCmd.Flags().String("","","...")
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

	_, lastAppliedMigration, err := db.FindLastAppliedMigration(ctx, migrations)
	if err != nil {
		return err
	}

	if lastAppliedMigration == nil {
		return errors.New("no migration has been applied yet")
	}

	return rockhopper.Redo(ctx, db, lastAppliedMigration)
}
