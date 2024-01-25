package rockhopper

import (
	"context"
)

func UpBySteps(ctx context.Context, db *DB, m *Migration, steps int, callbacks ...func(m *Migration)) error {
	for ; steps > 0 && m != nil; m = m.Next {
		descMigration("upgrading", m)

		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}

		steps--
	}

	return nil
}

func Upgrade(ctx context.Context, db *DB, migrations MigrationSlice) error {
	migrationMap := migrations.MapByPackage()
	for _, pkgMigrations := range migrationMap {
		pkgMigrations = pkgMigrations.Sort().Connect()

		_, lastAppliedMigration, err := db.FindLastAppliedMigration(ctx, pkgMigrations)
		if err != nil {
			return err
		}

		startMigration := pkgMigrations.Head()
		if lastAppliedMigration != nil {
			startMigration = lastAppliedMigration.Next
		}

		err = Up(ctx, db, startMigration, 0, func(m *Migration) {
			// log.Infof("migration %d is applied", m.Version)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// UpgradeFromGo runs the migration upgrades from the registered go-code migration
// parameter packageNames is the package list you want to filter
func UpgradeFromGo(ctx context.Context, db *DB, packageNames ...string) error {
	var migrations MigrationSlice
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return Upgrade(ctx, db, migrations.FilterPackage(packageNames))
}

// Up executes the Up methods from the given migration object
// and continues the upgrades to the latest migration.
func Up(ctx context.Context, db *DB, m *Migration, to int64, callbacks ...func(m *Migration)) error {
	for ; m != nil; m = m.Next {
		if to > 0 && m.Version > to {
			break
		}

		descMigration("upgrading", m)

		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}
	}

	return nil
}
