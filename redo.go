package rockhopper

import (
	"context"
)

func Redo(ctx context.Context, db *DB, m *Migration) error {
	if err := m.Down(ctx, db); err != nil {
		return err
	}

	return m.Up(ctx, db)
}
