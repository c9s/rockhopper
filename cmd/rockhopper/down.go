package main

import (
	"context"
	"fmt"

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

	loader := &rockhopper.SqlMigrationLoader{}
	migrations, err := loader.Load(config.MigrationsDir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	currentVersion, err := db.CurrentVersion(ctx, rockhopper.DefaultPackageName)
	if err != nil {
		return err
	}

	if currentVersion == 0 {
		return fmt.Errorf("no applied migration, can not downgrade")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if to > 0 {
		return rockhopper.Down(ctx, db, migrations, currentVersion, to, func(m *rockhopper.Migration) {
			log.Infof("migration %v is applied for downgrade", m.Version)
		})
	}
	if steps == 0 {
		steps = 1
	}

	return rockhopper.DownBySteps(ctx, db, migrations, currentVersion, steps, func(m *rockhopper.Migration) {
		log.Infof("migration %v is applied for downgrade", m.Version)
	})
}
