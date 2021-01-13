package rockhopper

import (
	"context"
)

func UpBySteps(ctx context.Context, db *DB, migrations MigrationSlice, from int64, steps int, callbacks ...func(m *Migration)) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err == ErrVersionNotFound {
		m = migrations[len(migrations)-1]
	} else if err != nil {
		return err
	} else if m != nil {
		m = m.Next
	}

	for ; steps > 0 && m != nil; steps-- {
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
	} else if m != nil {
		m = m.Next
	}

	for ; m != nil; {
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
