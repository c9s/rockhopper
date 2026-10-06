package rockhopper

import (
	"context"
)

// Redo rolls back and reapplies the specified migration.
func Redo(ctx context.Context, db *DB, m *Migration) error {
	if err := m.Down(ctx, db); err != nil {
		return err
	}

	return m.Up(ctx, db)
}
