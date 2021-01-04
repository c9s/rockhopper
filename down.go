package rockhopper

import "context"

func Down(ctx context.Context, db *DB, migrations MigrationSlice, from, to int64) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err == ErrVersionNotFound {
		m = migrations[len(migrations) - 1]
	} else if err != nil {
		return err
	}

	for {
		if to > 0 && m.Version < to {
			break
		}

		if err := m.Down(ctx, db); err != nil {
			return err
		}

		if m.Previous == nil {
			break
		}

		m = m.Previous
	}

	return nil
}

