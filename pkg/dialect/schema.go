package dialect

import (
	"fmt"
	"strings"
)

const (
	// PackageColumnName is the SQL column that records a migration's package.
	PackageColumnName = "package"
	// VersionIDColumnName is the SQL column that records a migration's version.
	VersionIDColumnName = "version_id"
	// IsAppliedColumnName is the SQL column that records whether a migration is applied.
	IsAppliedColumnName = "is_applied"
	// RecordIDColumnName is the SQL column that identifies a migration record.
	RecordIDColumnName = "id"
	// TimestampColumnName is the SQL column that stores a migration record's timestamp.
	TimestampColumnName = "tstamp"
	// DataMigrationStatusColumnName is the SQL column that records a data migration's status.
	DataMigrationStatusColumnName = "status"
	// DataMigrationCheckpointColumnName is the SQL column that stores a data migration's checkpoint.
	DataMigrationCheckpointColumnName = "checkpoint"
	// DataMigrationLeaseOwnerColumnName is the SQL column that stores a data migration lease owner.
	DataMigrationLeaseOwnerColumnName = "lease_owner"
	// DataMigrationLeaseExpiresAtColumnName is the SQL column that stores a data migration lease expiry.
	DataMigrationLeaseExpiresAtColumnName = "lease_expires_at"

	sqlTextType      = "TEXT"
	sqlTimestampType = "TIMESTAMP"
	sqlNowExpression = "NOW()"
)

// ColumnType is an abstract column type. Each dialect maps it to its own DDL
// spelling (see the ddlRenderer implementations in the per-dialect files).
type ColumnType int

const (
	// ColSerial is the auto-incrementing primary-key column. Its dialect spelling
	// carries the full definition (NOT NULL, identity/auto-increment, and, for
	// SQLite, the inline PRIMARY KEY).
	ColSerial ColumnType = iota
	// ColBigInt represents a signed large integer.
	ColBigInt
	// ColBool represents a boolean value.
	ColBool
	// ColVarchar represents a length-limited string.
	ColVarchar
	// ColText represents an unbounded string.
	ColText
	// ColTimestamp represents a timestamp value.
	ColTimestamp
)

// DefaultNow is a sentinel Column.Default meaning "the dialect's current
// timestamp expression" (NOW(), sysdate, datetime('now'), ...).
const DefaultNow = "__NOW__"

// Column describes one column of a table in dialect-neutral terms.
type Column struct {
	Name       string
	Type       ColumnType
	Size       int    // for ColVarchar
	NotNull    bool   // ignored for ColSerial (its spelling is self-contained)
	PrimaryKey bool   // ignored for ColSerial except to drive the PRIMARY KEY clause
	Default    string // literal default (e.g. "'main'", "0"), DefaultNow, or "" for none
}

// Schema describes a table so each dialect can render its own CREATE TABLE.
type Schema struct {
	Table   string
	Columns []Column
	Unique  [][]string // each inner slice is a UNIQUE column group
}

// ddlRenderer captures the per-dialect DDL choices used by buildCreateTable and
// buildAddColumn. Implemented (by hand) in each dialect file.
type ddlRenderer interface {
	// sqlType returns the physical type for a column. For ColSerial it returns
	// the full self-contained definition.
	sqlType(c Column) string
	// nowDefault returns the expression placed after DEFAULT for DefaultNow
	// columns (DDL context; may differ from Tokens.NowExpr, e.g. SQLite).
	nowDefault() string
	// ifNotExists reports whether CREATE TABLE IF NOT EXISTS is supported.
	ifNotExists() bool
	// inlinePK reports whether the serial column carries its PRIMARY KEY inline
	// (SQLite), so no separate PRIMARY KEY(...) clause is emitted.
	inlinePK() bool
}

func buildCreateTable(r ddlRenderer, s Schema) string {
	lines := make([]string, 0, len(s.Columns)+len(s.Unique)+1)
	var pkCols []string

	for _, col := range s.Columns {
		lines = append(lines, "    "+columnDefinition(r, col))
		if col.PrimaryKey && !r.inlinePK() {
			pkCols = append(pkCols, col.Name)
		}
	}

	if len(pkCols) > 0 {
		lines = append(lines, fmt.Sprintf("    PRIMARY KEY(%s)", strings.Join(pkCols, ", ")))
	}
	for _, u := range s.Unique {
		lines = append(lines, fmt.Sprintf("    UNIQUE(%s)", strings.Join(u, ", ")))
	}

	ifne := ""
	if r.ifNotExists() {
		ifne = "IF NOT EXISTS "
	}

	return fmt.Sprintf("CREATE TABLE %s%s (\n%s\n);", ifne, s.Table, strings.Join(lines, ",\n"))
}

func buildAddColumn(r ddlRenderer, table string, c Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, columnDefinition(r, c))
}

func columnDefinition(r ddlRenderer, c Column) string {
	if c.Type == ColSerial {
		return c.Name + " " + r.sqlType(c)
	}

	def := c.Name + " " + r.sqlType(c)
	if c.NotNull {
		def += " NOT NULL"
	}
	switch {
	case c.Default == DefaultNow:
		def += " DEFAULT " + r.nowDefault()
	case c.Default != "":
		def += " DEFAULT " + c.Default
	}
	return def
}
