package rockhopper

import (
	"context"
)

func UpBySteps(ctx context.Context, db *DB, migrations MigrationSlice, from int64, steps int, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m := migrations[0]
	if from > 0 {
		fromMigration, err := migrations.Find(from)
		if err != nil { // if from is given, ErrVersionNotFound could also be returned, and this should be treated as an error.
			return err
		}

		m = fromMigration.Next
	}

	for ; steps > 0 && m != nil; m = m.Next {
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

func Up(ctx context.Context, db *DB, migrations MigrationSlice, from, to int64, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m := migrations[0]
	if from > 0 {
		fromMigration, err := migrations.Find(from)
		if err != nil { // if from is given, ErrVersionNotFound could also be returned, and this should be treated as an error.
			return err
		}

		m = fromMigration.Next
	}

	for ; m != nil; m = m.Next {
		if to > 0 && m.Version > to {
			break
		}

		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}
	}

	return nil
}
