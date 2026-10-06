package rockhopper

import (
	"context"
)

// UpMigrations applies the given migrations in slice order. The migrations are
// expected to be pending (not yet applied) and sorted in ascending version order;
// any migration already marked as applied is skipped defensively.
//
// Unlike Up, which walks the Next linked-list pointers starting from a single
// migration, UpMigrations applies exactly the migrations in the slice. This makes
// it possible to apply out-of-order migrations — pending migrations whose version
// is lower than an already-applied one — which a Next-pointer walk would skip.
func UpMigrations(ctx context.Context, db *DB, migrations MigrationSlice, callbacks ...func(m *Migration)) error {
	for _, m := range migrations {
		if m.Record != nil && m.Record.IsApplied {
			continue
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

// UpBySteps applies at most steps migrations starting at m.
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

// Upgrade applies all pending migrations in each package.
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

		err = Up(ctx, db, startMigration, 0)

		if err != nil {
			return err
		}
	}

	return nil
}

// UpgradeFromGo applies registered Go migrations, optionally filtered by package name.
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
