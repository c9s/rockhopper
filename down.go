package rockhopper

import "context"

func DownBySteps(ctx context.Context, db *DB, migrations MigrationSlice, from int64, steps int, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err != nil { // if no migration is found, we should not continue
		return err
	}

	for ; steps > 0; steps-- {
		if err := m.Down(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}

		if m.Previous == nil {
			break
		}

		m = m.Previous
	}

	return nil
}

func Down(ctx context.Context, db *DB, migrations MigrationSlice, from, to int64, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err != nil { // if no migration is found, we should not continue
		return err
	}

	for ; m.Previous != nil; m = m.Previous {
		if to > 0 && m.Version < to {
			break
		}

		if err := m.Down(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}
	}

	return nil
}
