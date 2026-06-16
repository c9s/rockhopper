package migrations

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/app2/20240117132418_create_table_1.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("app2", 20240117132418, "migrations/mysql/app2/20240117132418_create_table_1.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE app2_a(a int);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE app2_a;"},
		},
	)
}
