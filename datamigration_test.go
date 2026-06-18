package rockhopper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pkCursor is the JSON-serialized checkpoint used by the test backfill: it
// advances an exclusive lower bound over an auto-increment primary key.
type pkCursor struct {
	Last int64 `json:"last"`
	Max  int64 `json:"max"`
}

// backfillMigrator marks every row in a table as migrated, batch.size rows at a
// time. failAfterBatch lets a test simulate a crash mid-migration.
type backfillMigrator struct {
	table          string
	batchSize      int64
	failAfterBatch int

	planCalls  int
	batchCalls int
}

func (b *backfillMigrator) Plan(ctx context.Context, q Queryer) (Checkpoint, error) {
	b.planCalls++

	var c pkCursor
	if err := q.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM "+b.table).Scan(&c.Max); err != nil {
		return nil, err
	}

	return json.Marshal(c)
}

func (b *backfillMigrator) Batch(ctx context.Context, exec BatchExecutor, cp Checkpoint) (Checkpoint, bool, error) {
	b.batchCalls++
	if b.failAfterBatch > 0 && b.batchCalls > b.failAfterBatch {
		return nil, false, fmt.Errorf("simulated crash at batch %d", b.batchCalls)
	}

	var c pkCursor
	if err := json.Unmarshal(cp, &c); err != nil {
		return nil, false, err
	}

	hi := c.Last + b.batchSize
	// idempotent: only touches rows in the (Last, hi] window that are not yet migrated.
	if _, err := exec.ExecContext(ctx,
		"UPDATE "+b.table+" SET migrated = 1 WHERE id > ? AND id <= ? AND migrated = 0", c.Last, hi); err != nil {
		return nil, false, err
	}

	c.Last = hi
	next, err := json.Marshal(c)
	if err != nil {
		return nil, false, err
	}

	return next, c.Last >= c.Max, nil
}

func openDataMigrationTestDB(t *testing.T) *DB {
	t.Helper()

	d, err := LoadDialect("sqlite3")
	require.NoError(t, err)

	db, err := Open("sqlite3", d, ":memory:", TableName)
	require.NoError(t, err)

	// :memory: gives a fresh database per connection; pin the pool to one
	// connection so all statements see the same database.
	db.SetMaxOpenConns(1)

	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, migrated INTEGER NOT NULL DEFAULT 0)`)
	require.NoError(t, err)

	return db
}

func seedUsers(t *testing.T, db *DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := db.Exec(`INSERT INTO users (migrated) VALUES (0)`)
		require.NoError(t, err)
	}
}

func countMigrated(t *testing.T, db *DB) int {
	t.Helper()
	var c int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE migrated = 1`).Scan(&c))
	return c
}

func mustCursor(t *testing.T, last, max int64) string {
	t.Helper()
	b, err := json.Marshal(pkCursor{Last: last, Max: max})
	require.NoError(t, err)
	return string(b)
}

// seedDataMigrationRow inserts a pre-existing state row, used to simulate
// another process having a lease.
func seedDataMigrationRow(t *testing.T, db *DB, dm *DataMigration, status, checkpoint, owner string, expiresAt int64) {
	t.Helper()
	require.NoError(t, db.TouchDataMigrationTable(context.Background()))
	_, err := db.Exec(
		"INSERT INTO "+DataMigrationTableName+" (package, version_id, name, status, checkpoint, lease_owner, lease_expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		dm.Package, dm.Version, dm.Name, status, checkpoint, owner, expiresAt)
	require.NoError(t, err)
}

func leaseState(t *testing.T, db *DB, dm *DataMigration) (owner sql.NullString, expiresAt int64, status string) {
	t.Helper()
	err := db.QueryRow(
		"SELECT lease_owner, lease_expires_at, status FROM "+DataMigrationTableName+" WHERE package = ? AND version_id = ?",
		dm.Package, dm.Version).Scan(&owner, &expiresAt, &status)
	require.NoError(t, err)
	return owner, expiresAt, status
}

func TestRunDataMigration_Backfill(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 25)

	mig := &backfillMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000001, Name: "backfill_users", Migrator: mig}

	require.NoError(t, RunDataMigration(ctx, db, dm))

	assert.Equal(t, 25, countMigrated(t, db))
	assert.Equal(t, 1, mig.planCalls)
	assert.Equal(t, 3, mig.batchCalls) // 10 + 10 + 5

	status, _, found, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, DataMigrationCompleted, status)
}

func TestRunDataMigration_SkipsWhenCompleted(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 5)

	mig := &backfillMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000002, Migrator: mig}

	require.NoError(t, RunDataMigration(ctx, db, dm))
	require.NoError(t, RunDataMigration(ctx, db, dm))

	// the second run must not call Plan or Batch again.
	assert.Equal(t, 1, mig.planCalls)
	assert.Equal(t, 1, mig.batchCalls)
}

func TestRunDataMigration_ResumesAfterFailure(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 25)

	// fail after the second committed batch (20 rows migrated, checkpoint at 20).
	mig := &backfillMigrator{table: "users", batchSize: 10, failAfterBatch: 2}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000003, Migrator: mig}

	err := RunDataMigration(ctx, db, dm)
	require.Error(t, err)

	status, _, found, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, DataMigrationFailed, status)
	assert.Equal(t, 20, countMigrated(t, db)) // two batches committed before the crash

	// resume: stop failing and run again.
	mig.failAfterBatch = 0
	require.NoError(t, RunDataMigration(ctx, db, dm))

	assert.Equal(t, 25, countMigrated(t, db))
	// Plan must not be called again on resume.
	assert.Equal(t, 1, mig.planCalls)

	status, _, _, err = db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	require.NoError(t, err)
	assert.Equal(t, DataMigrationCompleted, status)
}

func TestRunDataMigration_ReleasesLeaseOnComplete(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 5)

	mig := &backfillMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000020, Migrator: mig}

	require.NoError(t, RunDataMigration(ctx, db, dm))

	owner, expiresAt, status := leaseState(t, db, dm)
	assert.Equal(t, DataMigrationCompleted, status)
	assert.False(t, owner.Valid, "lease owner should be cleared after completion")
	assert.EqualValues(t, 0, expiresAt)
}

func TestRunDataMigration_LeaseHeldByAnother(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 25)

	mig := &backfillMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000021, Migrator: mig}

	// another process holds a live lease.
	future := time.Now().Add(1 * time.Hour).Unix()
	seedDataMigrationRow(t, db, dm, DataMigrationRunning, mustCursor(t, 0, 25), "other-pod", future)

	err := RunDataMigration(ctx, db, dm)
	assert.ErrorIs(t, err, ErrLeaseHeld)

	assert.Equal(t, 0, countMigrated(t, db))
	assert.Equal(t, 0, mig.planCalls)
	assert.Equal(t, 0, mig.batchCalls)

	// the other process's lease must be left untouched.
	owner, _, _ := leaseState(t, db, dm)
	assert.Equal(t, "other-pod", owner.String)
}

func TestRunDataMigration_StealsExpiredLease(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 25)

	mig := &backfillMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000022, Migrator: mig}

	// a dead process left an expired lease with a running status and checkpoint.
	past := time.Now().Add(-1 * time.Hour).Unix()
	seedDataMigrationRow(t, db, dm, DataMigrationRunning, mustCursor(t, 0, 25), "dead-pod", past)

	require.NoError(t, RunDataMigration(ctx, db, dm))

	assert.Equal(t, 25, countMigrated(t, db))
	// status was running (not pending), so we resume rather than re-Plan.
	assert.Equal(t, 0, mig.planCalls)

	owner, _, status := leaseState(t, db, dm)
	assert.Equal(t, DataMigrationCompleted, status)
	assert.False(t, owner.Valid)
}

// leaseStealingMigrator mutates the lease owner from inside a batch to simulate
// the lease being stolen mid-flight; the guarded commit then affects no rows.
type leaseStealingMigrator struct {
	table     string
	batchSize int64
}

func (b *leaseStealingMigrator) Plan(ctx context.Context, q Queryer) (Checkpoint, error) {
	var c pkCursor
	if err := q.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM "+b.table).Scan(&c.Max); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

func (b *leaseStealingMigrator) Batch(ctx context.Context, exec BatchExecutor, cp Checkpoint) (Checkpoint, bool, error) {
	var c pkCursor
	if err := json.Unmarshal(cp, &c); err != nil {
		return nil, false, err
	}

	hi := c.Last + b.batchSize
	if _, err := exec.ExecContext(ctx,
		"UPDATE "+b.table+" SET migrated = 1 WHERE id > ? AND id <= ?", c.Last, hi); err != nil {
		return nil, false, err
	}

	// steal the lease from under ourselves (single-row test table).
	if _, err := exec.ExecContext(ctx,
		"UPDATE "+DataMigrationTableName+" SET lease_owner = 'thief'"); err != nil {
		return nil, false, err
	}

	c.Last = hi
	next, err := json.Marshal(c)
	if err != nil {
		return nil, false, err
	}
	return next, c.Last >= c.Max, nil
}

func TestRunDataMigration_LeaseLostMidBatch(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 25)

	mig := &leaseStealingMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{Package: DefaultPackageName, Version: 1700000000000023, Migrator: mig}

	err := RunDataMigration(ctx, db, dm)
	assert.ErrorIs(t, err, ErrLeaseLost)

	// the in-flight batch was rolled back: no rows committed.
	assert.Equal(t, 0, countMigrated(t, db))
}

func TestAddNamedDataMigration(t *testing.T) {
	mig := &backfillMigrator{table: "users", batchSize: 10}
	AddNamedDataMigration("backfillpkg", "1700000000000010_backfill_users.go", mig,
		After(1699999999999999),
		WithThrottle(0),
		WithDataMigrationName("backfill users"),
	)

	got := DataMigrationsByPackage("backfillpkg")
	require.Len(t, got, 1)
	assert.Equal(t, int64(1700000000000010), got[0].Version)
	assert.Equal(t, int64(1699999999999999), got[0].After)
	assert.Equal(t, "backfill users", got[0].Name)
	assert.Same(t, mig, got[0].Migrator.(*backfillMigrator))
}

func TestRunDataMigration_DependencyGate(t *testing.T) {
	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 5)
	require.NoError(t, db.Touch(ctx))

	const schemaVersion = int64(1699999999999999)
	mig := &backfillMigrator{table: "users", batchSize: 10}
	dm := &DataMigration{
		Package:  DefaultPackageName,
		Version:  1700000000000004,
		Migrator: mig,
		After:    schemaVersion,
	}

	// schema dependency not applied yet -> refuse to run.
	err := RunDataMigration(ctx, db, dm)
	require.Error(t, err)
	assert.Equal(t, 0, countMigrated(t, db))

	// apply the schema version, then it should run.
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, db.insertVersion(ctx, tx, DefaultPackageName, "", schemaVersion, true))
	require.NoError(t, tx.Commit())

	require.NoError(t, RunDataMigration(ctx, db, dm))
	assert.Equal(t, 5, countMigrated(t, db))
}
