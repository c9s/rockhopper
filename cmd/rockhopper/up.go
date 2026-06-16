package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper/v2"
)

var log = logrus.WithField("application", "rockhopper")

func init() {
	UpCmd.Flags().Int64("to", 0, "up to a specific version")
	UpCmd.Flags().Int("steps", 0, "run upgrade by steps")
	UpCmd.Flags().Bool("allow-out-of-order", false, "apply pending migrations whose version is below an already-applied migration")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	allowOutOfOrder, err := cmd.Flags().GetBool("allow-out-of-order")
	if err != nil {
		return err
	}

	db, err := rockhopper.OpenWithConfig(config)
	if err != nil {
		return err
	}

	defer db.Close()

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

	debugMigrations(allMigrations)

	migrationMap := allMigrations.MapByPackage()

	if len(config.IncludePackages) > 0 {
		migrationMap = migrationMap.FilterPackage(config.IncludePackages)
	}

	migrationMap = migrationMap.SortAndConnect()

	for pkgName, migrations := range migrationMap {
		status, err := db.InspectMigrations(ctx, migrations)
		if err != nil {
			return err
		}

		if len(status.OutOfOrder) > 0 {
			if !allowOutOfOrder {
				return &rockhopper.OutOfOrderError{
					Package:               pkgName,
					HighestAppliedVersion: status.HighestAppliedVersion,
					Migrations:            status.OutOfOrder,
				}
			}

			for _, m := range status.OutOfOrder {
				log.Warnf("applying out-of-order migration %d (%s); it is older than the already-applied version %d",
					m.Version, m.Source, status.HighestAppliedVersion)
			}
		}

		target := selectPending(status.Pending, steps, to)
		if err := rockhopper.UpMigrations(ctx, db, target); err != nil {
			return err
		}
	}

	return nil
}

// selectPending narrows the pending migrations down to those that should be applied
// for this run. steps takes precedence over to: with steps > 0 it returns at most
// that many migrations; otherwise with to > 0 it returns those at or below the
// target version. The slice is assumed to be in ascending version order.
func selectPending(pending rockhopper.MigrationSlice, steps int, to int64) rockhopper.MigrationSlice {
	if steps > 0 {
		if steps < len(pending) {
			return pending[:steps]
		}
		return pending
	}

	if to > 0 {
		var selected rockhopper.MigrationSlice
		for _, m := range pending {
			if m.Version > to {
				break
			}
			selected = append(selected, m)
		}
		return selected
	}

	return pending
}
