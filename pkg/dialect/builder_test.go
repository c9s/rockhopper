package dialect

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/rockhopper/v2/internal/column"
)

const (
	testMySQLDialectName    = "mysql"
	testPostgresDialectName = "postgres"
	testSQLiteDialectName   = "sqlite3"
	testDefaultPackage      = "main"
	testLeaseOwner          = "owner-a"
)

// leaseCapable is a dialect that exposes both the core CRUD shapes and the
// data-migration lease shapes (the OLTP dialects).
type leaseCapable interface {
	Builder
	LeaseBuilder
}

// builderUnderTest pairs a human-readable name with a CRUD builder so each shape
// can be asserted against both the "?" and "$N" placeholder styles.
type builderUnderTest struct {
	name string
	b    leaseCapable
}

func builders() []builderUnderTest {
	return []builderUnderTest{
		{testMySQLDialectName, NewMySQLDialect()},       // "?" placeholders, NOW()
		{testPostgresDialectName, NewPostgresDialect()}, // "$N" placeholders, NOW()
	}
}

func TestCRUD_Insert(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			sql, args := c.b.Insert("t", []Col{{"a", 1}, {"b", 2}})
			if c.name == testMySQLDialectName {
				assert.Equal(t, "INSERT INTO t (a, b) VALUES (?, ?)", sql)
			} else {
				assert.Equal(t, "INSERT INTO t (a, b) VALUES ($1, $2)", sql)
			}
			assert.Equal(t, []any{1, 2}, args)
		})
	}
}

func TestCRUD_Delete(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			sql, args := c.b.Delete("t", []Col{{column.Package, "p"}, {column.VersionID, int64(7)}})
			if c.name == testMySQLDialectName {
				assert.Equal(t, "DELETE FROM t WHERE package = ? AND version_id = ?", sql)
			} else {
				assert.Equal(t, "DELETE FROM t WHERE package = $1 AND version_id = $2", sql)
			}
			assert.Equal(t, []any{"p", int64(7)}, args)
		})
	}
}

func TestCRUD_SelectOrderLimit(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			sql, args := c.b.Select("t",
				[]string{column.RecordID, column.Timestamp, column.IsApplied},
				[]Col{{column.Package, "p"}},
				SelectOpt{OrderBy: []Order{{Col: column.Timestamp, Desc: true}}, Limit: 1})
			if c.name == testMySQLDialectName {
				assert.Equal(t, "SELECT id, tstamp, is_applied FROM t WHERE package = ? ORDER BY tstamp DESC LIMIT 1", sql)
			} else {
				assert.Equal(t, "SELECT id, tstamp, is_applied FROM t WHERE package = $1 ORDER BY tstamp DESC LIMIT 1", sql)
			}
			assert.Equal(t, []any{"p"}, args)
		})
	}
}

func TestCRUD_SelectAggregateNoKeys(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			// Raw projection passes through unquoted; no WHERE when keys is empty.
			sql, args := c.b.Select("t", []string{"MAX(version_id)"}, nil, SelectOpt{})
			assert.Equal(t, "SELECT MAX(version_id) FROM t", sql)
			assert.Empty(t, args)
		})
	}
}

func TestCRUD_AcquireLease(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			sql, args := c.b.AcquireLease("t",
				[]Col{{column.VersionID, int64(5)}},
				testLeaseOwner, int64(200), int64(100))
			if c.name == testMySQLDialectName {
				assert.Equal(t,
					"UPDATE t SET lease_owner = ?, lease_expires_at = ?, updated_at = NOW() "+
						"WHERE version_id = ? AND (lease_owner IS NULL OR lease_owner = ? OR lease_expires_at < ?)",
					sql)
			} else {
				assert.Equal(t,
					"UPDATE t SET lease_owner = $1, lease_expires_at = $2, updated_at = NOW() "+
						"WHERE version_id = $3 AND (lease_owner IS NULL OR lease_owner = $4 OR lease_expires_at < $5)",
					sql)
			}
			assert.Equal(t, []any{testLeaseOwner, int64(200), int64(5), testLeaseOwner, int64(100)}, args)
		})
	}
}

func TestCRUD_CommitLease(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			sql, args := c.b.CommitLease("t",
				[]Col{{column.Status, "running"}, {column.Checkpoint, "cp"}},
				[]Col{{column.VersionID, int64(5)}},
				testLeaseOwner)
			if c.name == testMySQLDialectName {
				assert.Equal(t,
					"UPDATE t SET status = ?, checkpoint = ?, updated_at = NOW() "+
						"WHERE version_id = ? AND lease_owner = ?",
					sql)
			} else {
				assert.Equal(t,
					"UPDATE t SET status = $1, checkpoint = $2, updated_at = NOW() "+
						"WHERE version_id = $3 AND lease_owner = $4",
					sql)
			}
			assert.Equal(t, []any{"running", "cp", int64(5), testLeaseOwner}, args)
		})
	}
}

func TestCRUD_ReleaseLease(t *testing.T) {
	for _, c := range builders() {
		t.Run(c.name, func(t *testing.T) {
			sql, args := c.b.ReleaseLease("t", "completed",
				[]Col{{column.VersionID, int64(5)}},
				testLeaseOwner)
			if c.name == testMySQLDialectName {
				assert.Equal(t,
					"UPDATE t SET status = ?, lease_expires_at = ?, updated_at = NOW(), lease_owner = NULL "+
						"WHERE version_id = ? AND lease_owner = ?",
					sql)
			} else {
				assert.Equal(t,
					"UPDATE t SET status = $1, lease_expires_at = $2, updated_at = NOW(), lease_owner = NULL "+
						"WHERE version_id = $3 AND lease_owner = $4",
					sql)
			}
			assert.Equal(t, []any{"completed", int64(0), int64(5), testLeaseOwner}, args)
		})
	}
}
