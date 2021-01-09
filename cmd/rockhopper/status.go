package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	RootCmd.AddCommand(StatusCmd)
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
	if err := checkConfig(config) ; err != nil {
		return err
	}

	dialectName := config.Dialect
	if len(dialectName) == 0 {
		dialectName = config.Driver
	}

	dialect, err := rockhopper.LoadDialect(dialectName)
	if err != nil {
		return err
	}

	db, err := rockhopper.Open(config.Driver, config.DSN, dialect)
	if err != nil {
		return err
	}

	loader := &rockhopper.SqlMigrationLoader{}
	migrations, err := loader.Load(config.MigrationsDir)
	if err != nil {
		return err
	}

	log.Println("    Applied At                  Migration")
	log.Println("    =======================================")
	for _, migration := range migrations {
		if err := printMigrationStatus(db, migration); err != nil {
			return errors.Wrap(err, "failed to print status")
		}
	}

	return nil
}

func printMigrationStatus(db *rockhopper.DB, migration *rockhopper.Migration) error {
	var script = filepath.Base(migration.Source)

	row, err := db.FindMigration(migration.Version)
	if err != nil {
		return err
	}

	var appliedAt string

	if row != nil && row.IsApplied {
		appliedAt = row.Time.Format(time.ANSIC)
	} else {
		appliedAt = "Pending"
	}

	log.Printf("    %-24s -- %v\n", appliedAt, script)
	return nil
}
