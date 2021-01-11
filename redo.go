package rockhopper

import (
	"context"
	"fmt"
)

func Redo(ctx context.Context, db *DB, from int64, migrations MigrationSlice) error {
	if len(migrations) == 0 {
		return nil
	}

	m, err := migrations.Find(from)
	if err == ErrVersionNotFound {
		return fmt.Errorf("no applied migration yet")
	} else if err != nil {
		return err
	} else if m == nil {
		return fmt.Errorf("migration not found")
	}

	if err := m.Down(ctx, db); err != nil {
		return err
	}

	if err := m.Up(ctx, db); err != nil {
		return err
	}

	return nil

}
