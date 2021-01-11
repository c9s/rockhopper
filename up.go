package rockhopper

import (
	"context"

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

func UpByStep(ctx context.Context, db *DB, migrations MigrationSlice, from int64, steps int, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err == ErrVersionNotFound {
		m = migrations[len(migrations)-1]
	} else if err != nil {
		return err
	}

	for ; steps >= 0; steps-- {
		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}

		if m.Next == nil {
			break
		}

		m = m.Next
	}

	return nil
}

func Up(ctx context.Context, db *DB, migrations MigrationSlice, from, to int64, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err == ErrVersionNotFound {
		m = migrations[0]
	} else if err != nil {
		return err
	}

	for {
		if to > 0 && m.Version > to {
			break
		}

		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}

		if m.Next == nil {
			break
		}

		m = m.Next
	}

	return nil
}
