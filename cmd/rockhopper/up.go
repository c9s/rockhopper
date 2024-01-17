package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

var log = logrus.WithField("application", "rockhopper")

func init() {
	UpCmd.Flags().Int64("to", 0, "up to a specific version")
	UpCmd.Flags().Int("steps", 0, "run upgrade by steps")
	rootCmd.AddCommand(UpCmd)
}

var UpCmd = &cobra.Command{
	Use:   "up",
	Short: "run migration scripts to upgrade database schema",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         up,
}

func up(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	steps, err := cmd.Flags().GetInt("steps")
	if err != nil {
		return err
	}

	to, err := cmd.Flags().GetInt64("to")
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

	migrationMap := allMigrations.MapByPackage()

	if len(config.IncludePackages) > 0 {
		migrationMap = migrationMap.FilterPackage(config.IncludePackages)
	}

	migrationMap = migrationMap.SortAndConnect()

	for pkgName, migrations := range migrationMap {
		_ = pkgName

		_, lastAppliedMigration, err := db.FindLastAppliedMigration(ctx, migrations)
		if err != nil {
			return err
		}

		startMigration := migrations.Head()
		if lastAppliedMigration != nil {
			startMigration = lastAppliedMigration.Next
		}

		if steps > 0 {
			return rockhopper.UpBySteps(ctx, db, startMigration, steps, func(m *rockhopper.Migration) {
				log.Infof("migration %v is applied", m.Version)
			})
		}

		return rockhopper.Up(ctx, db, startMigration, to, func(m *rockhopper.Migration) {
			log.Infof("migration %d is applied", m.Version)
		})
	}

	return nil
}
