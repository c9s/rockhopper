package dialect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCapability_LeaseBuilder pins which dialects advertise the data-migration
// lease capability. The OLTP dialects must; ClickHouse (OLAP) must not.
func TestCapability_LeaseBuilder(t *testing.T) {
	oltp := []struct {
		name string
		d    Dialect
	}{
		{"mysql", NewMySQLDialect()},
		{"postgres", NewPostgresDialect()},
		{"sqlite3", NewSqlite3Dialect()},
		{"tidb", NewTiDBDialect()},
		{"redshift", NewRedshiftDialect()},
	}
	for _, c := range oltp {
		_, ok := c.d.(LeaseBuilder)
		assert.Truef(t, ok, "%s must implement LeaseBuilder", c.name)
	}

	_, ok := Dialect(NewClickHouseDialect()).(LeaseBuilder)
	assert.False(t, ok, "ClickHouse must NOT implement LeaseBuilder")
}

func TestClickHouse_Insert(t *testing.T) {
	sql, args := NewClickHouseDialect().Insert("t", []Col{{"package", "main"}, {"version_id", int64(1)}})
	assert.Equal(t, "INSERT INTO t (package, version_id) VALUES ($1, $2)", sql)
	assert.Equal(t, []any{"main", int64(1)}, args)
}

// TestClickHouse_Delete asserts the mutation form: ClickHouse has no synchronous
// row DELETE.
func TestClickHouse_Delete(t *testing.T) {
	sql, args := NewClickHouseDialect().Delete("t", []Col{{"package", "main"}, {"version_id", int64(1)}})
	assert.Equal(t,
		"ALTER TABLE t DELETE WHERE package = $1 AND version_id = $2 SETTINGS mutations_sync = 2",
		sql)
	assert.Equal(t, []any{"main", int64(1)}, args)
}

func TestClickHouse_Select(t *testing.T) {
	sql, args := NewClickHouseDialect().Select("t",
		[]string{"package", "version_id", "is_applied", "tstamp"},
		[]Col{{"package", "main"}},
		SelectOpt{OrderBy: []Order{{Col: "id", Desc: true}}})
	assert.Equal(t,
		"SELECT package, version_id, is_applied, tstamp FROM t WHERE package = $1 ORDER BY id DESC",
		sql)
	assert.Equal(t, []any{"main"}, args)
}

func TestClickHouse_CreateTable(t *testing.T) {
	s := Schema{
		Table: "rockhopper_versions",
		Columns: []Column{
			{Name: "id", Type: ColSerial, PrimaryKey: true},
			{Name: "package", Type: ColVarchar, Size: 128, NotNull: true, Default: "'main'"},
			{Name: "source_file", Type: ColVarchar, Size: 255, NotNull: true, Default: "''"},
			{Name: "version_id", Type: ColBigInt, NotNull: true},
			{Name: "is_applied", Type: ColBool, NotNull: true},
			{Name: "tstamp", Type: ColTimestamp, NotNull: true, Default: DefaultNow},
		},
	}
	out := NewClickHouseDialect().CreateTable(s)

	assert.Contains(t, out, "CREATE TABLE IF NOT EXISTS rockhopper_versions")
	// no auto-increment: id is a monotonic nanosecond Int64.
	assert.Contains(t, out, "id Int64 DEFAULT toUnixTimestamp64Nano(now64(9))")
	assert.Contains(t, out, "package String DEFAULT 'main'")
	assert.Contains(t, out, "source_file String DEFAULT ''")
	assert.Contains(t, out, "version_id Int64")
	assert.Contains(t, out, "is_applied Bool")
	assert.Contains(t, out, "tstamp DateTime DEFAULT now()")
	assert.Contains(t, out, "ENGINE = MergeTree()")
	assert.Contains(t, out, "ORDER BY (package, version_id)")
	// MergeTree uses ORDER BY, not PRIMARY KEY; ClickHouse has no VARCHAR.
	assert.NotContains(t, out, "PRIMARY KEY")
	assert.NotContains(t, out, "VARCHAR")
}

func TestClickHouse_AddColumn(t *testing.T) {
	sql, ok := NewClickHouseDialect().AddColumn("goose_db_version",
		Column{Name: "package", Type: ColVarchar, Size: 128, NotNull: true, Default: "'main'"})
	assert.True(t, ok)
	assert.Equal(t, "ALTER TABLE goose_db_version ADD COLUMN package String DEFAULT 'main'", sql)
}

func TestClickHouse_TableNames(t *testing.T) {
	assert.Equal(t,
		"SELECT name FROM system.tables WHERE database = currentDatabase()",
		NewClickHouseDialect().TableNames())
}
