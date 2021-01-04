package rockhopper

import (
	"context"
	"database/sql"

	"github.com/spf13/viper"
)

func Run() error {
	migrationDir := viper.GetString("migrationDir")

	driver := viper.GetString("driver")
	dsn := viper.GetString("dsn")

	dialect, err := LoadDialect(driver)
	if err != nil {
		return err
	}

	db, err := Open(driver, dsn, dialect)
	if err != nil {
		return err
	}

	currentVersion, err := db.CurrentVersion()
	if err != nil {
		return err
	}

	_ = currentVersion

	var loader = &SqlMigrationLoader{}
	migrations, err := loader.Load(migrationDir)
	if err != nil {
		return err
	}

	m, err := migrations.Find(currentVersion)
	if err != nil {
		return err
	}

	for {
		if m.Next == nil {
			break
		}

		// m.Up()

		m = m.Next
	}

	return nil
}



// UpTo migrates up to a specific version.
func UpTo(ctx context.Context, db *sql.DB, migrations MigrationSlice, version int64) error {
	// migrations, err := CollectMigrations(dir, minVersion, version)
	/*
	for {
		current, err := GetDBVersion(db)
		if err != nil {
			return err
		}

		next, err := migrations.Next
		if err != nil {
			if err == ErrNoNextVersion {
				log.Printf("goose: no migrations to run. current version: %d\n", current)
				return nil
			}
			return err
		}

		if err = next.Up(ctx, db); err != nil {
			return err
		}
	}
	*/
	return nil
}

