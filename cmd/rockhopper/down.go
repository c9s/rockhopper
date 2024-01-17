package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	DownCmd.Flags().Int64("to", 0, "downgrade to a specific version")
	DownCmd.Flags().Int("steps", 0, "downgrade by steps")
	rootCmd.AddCommand(DownCmd)
}

var DownCmd = &cobra.Command{
	Use:   "down",
	Short: "downgrade database",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         down,
}

func down(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	to, err := cmd.Flags().GetInt64("to")
	if err != nil {
		return err
	}

	steps, err := cmd.Flags().GetInt("steps")
	if err != nil {
		return err
	}

	db, err := rockhopper.OpenWithConfig(config)
	if err != nil {
		return err
	}

	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := db.Touch(ctx); err != nil {
		return err
	}

	loader := rockhopper.NewSqlMigrationLoader(config)

	allMigrations, err := loader.Load(config.MigrationsDirs...)
	if err != nil {
		return err
	}

	if len(allMigrations) == 0 {
		log.Infof("no migrations found")
		return nil
	}

	log.Infof("loaded %d migrations", len(allMigrations))

	allMigrations = allMigrations.SortAndConnect()

	idx, lastAppliedMigration, err := db.FindLastAppliedMigration(ctx, allMigrations)
	if err != nil {
		return err
	}

	_ = idx

	if to > 0 {
		return rockhopper.Down(ctx, db, lastAppliedMigration, to, func(m *rockhopper.Migration) {
			log.Infof("migration %v is applied for downgrade", m.Version)
		})
	}

	if steps == 0 {
		steps = 1
	}

	return rockhopper.DownBySteps(ctx, db, lastAppliedMigration, steps, func(m *rockhopper.Migration) {
		log.Infof("migration %v is applied for downgrade", m.Version)
	})
}
