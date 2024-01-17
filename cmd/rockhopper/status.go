package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	rootCmd.AddCommand(StatusCmd)
}

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show migration status",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         status,
}

func checkConfig(config *rockhopper.Config) error {
	if config == nil {
		return fmt.Errorf("config is not loaded")
	}

	if len(config.Driver) == 0 {
		return fmt.Errorf("driver name can not be empty")
	}

	return nil
}

func status(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	db, err := rockhopper.OpenWithConfig(config)
	if err != nil {
		return err
	}

	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = db.CurrentVersion(ctx, rockhopper.DefaultPackageName)
	if err != nil {
		return err
	}

	loader := rockhopper.NewSqlMigrationLoader(config)

	allMigrations, err := loader.Load(config.MigrationsDirs...)
	if err != nil {
		return err
	}

	debugMigrations(allMigrations)

	if len(allMigrations) == 0 {
		log.Infof("no migrations found")
		return nil
	}

	log.Debugf("loaded %d migrations", len(allMigrations))

	migrationMap := allMigrations.MapByPackage()

	if len(config.IncludePackages) > 0 {
		migrationMap = migrationMap.FilterPackage(config.IncludePackages)
	}

	migrationMap = migrationMap.SortAndConnect()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Package", "Migration", "Applied At", "Current"})

	for pkgName, migrations := range migrationMap {
		currentVersion, err := db.CurrentVersion(ctx, pkgName)
		if err != nil {
			return err
		}

		for _, migration := range migrations {
			record, err := db.FindMigration(migration.Version)
			if err != nil {
				return err
			}

			t.AppendRow(table.Row{
				migration.Package, migration.Source, formatAppliedAt(record), currentVersionMark(migration.Version, currentVersion),
			})
		}

		t.AppendSeparator()
		_ = currentVersion
	}

	// t.AppendFooter(table.Row{"", "", "Total", 10000})
	t.Render()

	return nil
}

func debugMigrations(slice rockhopper.MigrationSlice) {
	for _, m := range slice {
		log.Debugf("loaded migration: %s %d <- %s", m.Package, m.Version, m.Source)
	}
}

func currentVersionMark(migrationVersion, currentVersion int64) string {
	if migrationVersion == currentVersion {
		return "*"
	}
	return "-"
}

func formatAppliedAt(row *rockhopper.MigrationRecord) string {
	var appliedAt = "pending"
	if row != nil && row.IsApplied {
		appliedAt = row.Time.Format(time.ANSIC)
	}

	return appliedAt
}
