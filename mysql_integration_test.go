//go:build !no_mysql
// +build !no_mysql

package rockhopper

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/rockhopper/v2/pkg/dockermanage"
	"github.com/c9s/rockhopper/v2/pkg/dockermanage/dockermysql"
)

// resolveMySQLDSN returns a DSN for the MySQL integration test.
//
// Resolution order:
//  1. TEST_MYSQL_DSN, if set, is used directly. This is how CI connects to a
//     service container, where the Docker API is not available to the job.
//  2. Otherwise a throwaway MySQL container is started via Docker (dockermysql),
//     and torn down when the test finishes.
//  3. When neither a DSN nor Docker is available, the test is skipped.
func resolveMySQLDSN(t *testing.T) string {
	t.Helper()

	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		t.Log("using TEST_MYSQL_DSN for the mysql integration test")
		return dsn
	}

	manager, err := dockermanage.NewManager()
	if err != nil {
		t.Skipf("skipping mysql integration test: TEST_MYSQL_DSN is unset and docker is unavailable: %v", err)
	}

	t.Log("TEST_MYSQL_DSN is unset; starting a throwaway mysql container via docker...")
	inst, err := dockermysql.Start(manager)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := inst.Purge(); err != nil {
			t.Logf("failed to purge mysql container: %v", err)
		}
	})

	return inst.DSN()
}

// tableExists reports whether the given table exists in the current database.
func tableExists(ctx context.Context, t *testing.T, db *DB, table string) bool {
	t.Helper()

	var count int
	row := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
		table)
	require.NoError(t, row.Scan(&count))
	return count > 0
}

// TestMySQLIntegration_StatusUpDown exercises the full status -> up -> down flow
// against a real MySQL instance (a CI service container or a local Docker
// container) using migration files that cover timestamp, datetime, boolean and
// decimal column types.
func TestMySQLIntegration_StatusUpDown(t *testing.T) {
	const pkg = "integration"

	dsn := resolveMySQLDSN(t)

	config := &Config{
		Driver:          DialectMySQL,
		Dialect:         DialectMySQL,
		DSN:             dsn,
		MigrationsDirs:  []string{"testdata/integration/mysql"},
		IncludePackages: []string{pkg},
	}

	db, err := OpenWithConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Start from a clean slate so the test is repeatable even if a previous run
	// (or a reused CI database) left tables behind. Drop in FK-safe order.
	cleanup := func() {
		for _, q := range []string{
			"DROP TABLE IF EXISTS orders",
			"DROP TABLE IF EXISTS products",
			"DROP TABLE IF EXISTS users",
			"DROP TABLE IF EXISTS " + TableName,
		} {
			_, _ = db.ExecContext(ctx, q)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	require.NoError(t, db.Touch(ctx))

	loader := NewSqlMigrationLoader(config)
	allMigrations, err := loader.Load(config.MigrationsDirs...)
	require.NoError(t, err)
	require.NotEmpty(t, allMigrations)

	migrationMap := allMigrations.MapByPackage().FilterPackage(config.IncludePackages).SortAndConnect()
	migrations, ok := migrationMap[pkg]
	require.True(t, ok, "expected migrations for package %q", pkg)
	require.Len(t, migrations, 3)

	lastVersion := migrations[len(migrations)-1].Version

	// --- status: everything pending before up ---
	status, err := db.InspectMigrations(ctx, migrations)
	require.NoError(t, err)
	assert.Len(t, status.Pending, 3, "all migrations should be pending before up")
	assert.Empty(t, status.OutOfOrder)
	assert.EqualValues(t, 0, status.HighestAppliedVersion)

	current, err := db.CurrentVersion(ctx, pkg)
	require.NoError(t, err)
	assert.EqualValues(t, 0, current, "current version should be 0 before up")

	// --- up: apply all pending migrations ---
	require.NoError(t, UpMigrations(ctx, db, status.Pending))

	for _, table := range []string{"users", "products", "orders"} {
		assert.True(t, tableExists(ctx, t, db, table), "table %q should exist after up", table)
	}

	current, err = db.CurrentVersion(ctx, pkg)
	require.NoError(t, err)
	assert.Equal(t, lastVersion, current, "current version should be the last migration after up")

	// --- status: nothing pending after up ---
	applied, err := db.InspectMigrations(ctx, migrations)
	require.NoError(t, err)
	assert.Empty(t, applied.Pending, "no migrations should be pending after up")
	assert.Equal(t, lastVersion, applied.HighestAppliedVersion)

	// Sanity check that the migrated schema actually accepts a row exercising
	// the boolean / decimal / datetime columns.
	_, err = db.ExecContext(ctx,
		"INSERT INTO users (name, email, is_active, balance, signup_at) VALUES (?, ?, ?, ?, NOW())",
		"alice", "alice@example.com", true, "12.34567890")
	require.NoError(t, err, "inserting into the migrated users table should succeed")

	// --- down: roll everything back from the last migration ---
	tail := migrations[len(migrations)-1]
	require.NoError(t, Down(ctx, db, tail, 0))

	for _, table := range []string{"users", "products", "orders"} {
		assert.False(t, tableExists(ctx, t, db, table), "table %q should be dropped after down", table)
	}

	current, err = db.CurrentVersion(ctx, pkg)
	require.NoError(t, err)
	assert.EqualValues(t, 0, current, "current version should be 0 after a full rollback")
}
