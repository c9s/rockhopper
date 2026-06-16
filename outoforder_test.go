package rockhopper

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMigration(version int64, createSQL, dropSQL string) *Migration {
	return &Migration{
		Package:        "main",
		Version:        version,
		Source:         "migrations/main/test.sql",
		UseTx:          true,
		UpStatements:   []Statement{{Direction: DirectionUp, SQL: createSQL}},
		DownStatements: []Statement{{Direction: DirectionDown, SQL: dropSQL}},
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()

	dialect, err := LoadDialect("sqlite3")
	require.NoError(t, err)

	db, err := Open("sqlite3", dialect, ":memory:", TableName)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.Touch(context.Background()))
	return db
}

// TestInspectMigrations_DetectsOutOfOrder simulates a teammate merging a migration
// with a version below one that is already applied, and asserts it is reported as
// pending *and* out of order (the silent-skip hazard).
func TestInspectMigrations_DetectsOutOfOrder(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	v1 := newTestMigration(20240101000000, "CREATE TABLE t1 (id INT)", "DROP TABLE t1")
	v2 := newTestMigration(20240102000000, "CREATE TABLE t2 (id INT)", "DROP TABLE t2")
	v3 := newTestMigration(20240103000000, "CREATE TABLE t3 (id INT)", "DROP TABLE t3")

	// v2 does not exist yet: apply v1 and v3.
	require.NoError(t, UpMigrations(ctx, db, MigrationSlice{v1, v3}))

	// v2 is merged later; inspect the full set.
	full := MigrationSlice{v1, v2, v3}.SortAndConnect()
	status, err := db.InspectMigrations(ctx, full)
	require.NoError(t, err)

	assert.Equal(t, int64(20240103000000), status.HighestAppliedVersion)
	require.Len(t, status.Pending, 1)
	assert.Equal(t, int64(20240102000000), status.Pending[0].Version)
	require.Len(t, status.OutOfOrder, 1)
	assert.Equal(t, int64(20240102000000), status.OutOfOrder[0].Version)

	// The typed error should carry the offending migration and be actionable.
	ooErr := &OutOfOrderError{
		Package:               "main",
		HighestAppliedVersion: status.HighestAppliedVersion,
		Migrations:            status.OutOfOrder,
	}
	msg := ooErr.Error()
	assert.Contains(t, msg, "out-of-order")
	assert.Contains(t, msg, "20240102000000")
	assert.Contains(t, msg, "--allow-out-of-order")

	// Applying the out-of-order migration clears the hazard.
	require.NoError(t, UpMigrations(ctx, db, status.OutOfOrder))

	status2, err := db.InspectMigrations(ctx, full)
	require.NoError(t, err)
	assert.Empty(t, status2.Pending)
	assert.Empty(t, status2.OutOfOrder)
}

// TestInspectMigrations_InOrderHasNoFalsePositive ensures a normal forward sequence
// (pending migrations all above the highest applied) is never flagged out of order.
func TestInspectMigrations_InOrderHasNoFalsePositive(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	v1 := newTestMigration(20240101000000, "CREATE TABLE t1 (id INT)", "DROP TABLE t1")
	v2 := newTestMigration(20240102000000, "CREATE TABLE t2 (id INT)", "DROP TABLE t2")
	v3 := newTestMigration(20240103000000, "CREATE TABLE t3 (id INT)", "DROP TABLE t3")

	require.NoError(t, UpMigrations(ctx, db, MigrationSlice{v1}))

	full := MigrationSlice{v1, v2, v3}.SortAndConnect()
	status, err := db.InspectMigrations(ctx, full)
	require.NoError(t, err)

	assert.Equal(t, int64(20240101000000), status.HighestAppliedVersion)
	assert.Equal(t, []int64{20240102000000, 20240103000000}, status.Pending.Versions())
	assert.Empty(t, status.OutOfOrder, "forward-only pending migrations must not be flagged out of order")
}

// TestInspectMigrations_NothingApplied treats every migration as pending and never
// out of order when the database is empty.
func TestInspectMigrations_NothingApplied(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	full := MigrationSlice{
		newTestMigration(20240101000000, "CREATE TABLE t1 (id INT)", "DROP TABLE t1"),
		newTestMigration(20240102000000, "CREATE TABLE t2 (id INT)", "DROP TABLE t2"),
	}.SortAndConnect()

	status, err := db.InspectMigrations(ctx, full)
	require.NoError(t, err)

	assert.Equal(t, int64(0), status.HighestAppliedVersion)
	assert.Len(t, status.Pending, 2)
	assert.Empty(t, status.OutOfOrder)
}

func TestOutOfOrderError_Message(t *testing.T) {
	err := &OutOfOrderError{
		Package:               "billing",
		HighestAppliedVersion: 20240105000000,
		Migrations: MigrationSlice{
			newTestMigration(20240102000000, "", ""),
		},
	}

	msg := err.Error()
	assert.True(t, strings.Contains(msg, "billing"))
	assert.True(t, strings.Contains(msg, "20240105000000"))
	assert.True(t, strings.Contains(msg, "20240102000000"))
}
