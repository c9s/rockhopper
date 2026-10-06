package rockhopper

import "context"

// DownBySteps rolls back at most steps migrations starting at m.
func DownBySteps(ctx context.Context, db *DB, m *Migration, steps int, callbacks ...func(m *Migration)) error {
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

// Down rolls back migrations from m until the target version is reached.
func Down(ctx context.Context, db *DB, m *Migration, to int64, callbacks ...func(m *Migration)) error {
	for ; m != nil; m = m.Previous {
		if to > 0 && m.Version <= to {
			break
		}

		descMigration("downgrading", m)

		if err := m.Down(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}
	}

	return nil
}
