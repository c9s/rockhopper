package migrations

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("app2", up_app2_createTable_1, down_app2_createTable_1)

}

func up_app2_createTable_1(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE app2_a(a int);")
	if err != nil {
		return err
	}
	return err
}

func down_app2_createTable_1(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE app2_a;")
	if err != nil {
		return err
	}
	return err
}
