package dialect

import (
	"fmt"
	"strings"
)

// ClickHouseDialect implements Dialect for ClickHouse.
//
// ClickHouse is an OLAP column store and diverges from the OLTP dialects in
// several ways that this dialect encapsulates:
//
//   - It has no auto-increment: the surrogate id column defaults to a monotonic
//     nanosecond timestamp (toUnixTimestamp64Nano(now64(9))), which stays Int64
//     and preserves the "ORDER BY id DESC = most recent record first" semantics
//     that rockhopper relies on.
//   - Tables use a MergeTree engine with an explicit ORDER BY key instead of a
//     PRIMARY KEY clause, and constraints such as UNIQUE are not enforced.
//   - Rows are removed with an asynchronous ALTER TABLE ... DELETE mutation, not a
//     synchronous DELETE.
//
// Because ClickHouse cannot honor a conditional UPDATE whose RowsAffected()==1
// signals exclusive ownership, it embeds plain CRUD (not LeaseCRUD) and therefore
// does NOT satisfy LeaseBuilder; the data-migration layer detects this and refuses
// to run data migrations on ClickHouse.
type ClickHouseDialect struct {
	CRUD
}

// NewClickHouseDialect constructs a ClickHouseDialect with its CRUD builder wired
// to its own tokens.
func NewClickHouseDialect() *ClickHouseDialect {
	d := &ClickHouseDialect{}
	d.CRUD = NewCRUD(d)
	return d
}

func (d *ClickHouseDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (d *ClickHouseDialect) NowExpr() string          { return "now()" }

func (d *ClickHouseDialect) TableNames() string {
	return "SELECT name FROM system.tables WHERE database = currentDatabase()"
}

// Delete overrides the generic shape: ClickHouse has no synchronous row DELETE,
// so deletions go through an ALTER TABLE ... DELETE mutation. mutations_sync = 2
// waits for the mutation to finish (on all replicas) before returning, so the row
// is gone by the time the call completes.
func (d *ClickHouseDialect) Delete(table string, keys []Col) (string, []any) {
	where, args := d.eqClauses(keys, 0, " AND ")
	return fmt.Sprintf("ALTER TABLE %s DELETE WHERE %s SETTINGS mutations_sync = 2", table, where), args
}

func (d *ClickHouseDialect) CreateTable(s Schema) string { return buildClickHouseCreateTable(s) }

func (d *ClickHouseDialect) AddColumn(table string, c Column) (string, bool) {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, clickhouseColumnDef(c)), true
}

func buildClickHouseCreateTable(s Schema) string {
	lines := make([]string, 0, len(s.Columns))
	for _, c := range s.Columns {
		lines = append(lines, "    "+clickhouseColumnDef(c))
	}

	// MergeTree requires an explicit sorting key; UNIQUE constraints are not
	// enforced by ClickHouse and are intentionally dropped here.
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n)\nENGINE = MergeTree()\nORDER BY %s",
		s.Table, strings.Join(lines, ",\n"), clickhouseOrderBy(s))
}

func clickhouseColumnDef(c Column) string {
	// ColSerial carries its own DEFAULT, so it is self-contained.
	if c.Type == ColSerial {
		return c.Name + " " + clickhouseType(c)
	}

	def := c.Name + " " + clickhouseType(c)
	switch {
	case c.Default == DefaultNow:
		def += " DEFAULT now()"
	case c.Default != "":
		def += " DEFAULT " + c.Default
	}
	return def
}

func clickhouseType(c Column) string {
	switch c.Type {
	case ColSerial:
		// No auto-increment: a monotonic nanosecond timestamp keeps id an Int64
		// while preserving insertion order for "ORDER BY id DESC".
		return "Int64 DEFAULT toUnixTimestamp64Nano(now64(9))"
	case ColBigInt:
		return "Int64"
	case ColBool:
		return "Bool"
	case ColVarchar:
		return "String"
	case ColText:
		return "String"
	case ColTimestamp:
		return "DateTime"
	}
	return ""
}

// clickhouseOrderBy derives the MergeTree sorting key. rockhopper's tables are
// keyed by (package, version_id); when those columns are absent it falls back to
// the first column, or an empty tuple for a column-less table.
func clickhouseOrderBy(s Schema) string {
	has := func(name string) bool {
		for _, c := range s.Columns {
			if c.Name == name {
				return true
			}
		}
		return false
	}

	var keys []string
	for _, name := range []string{"package", "version_id"} {
		if has(name) {
			keys = append(keys, name)
		}
	}

	if len(keys) == 0 {
		if len(s.Columns) == 0 {
			return "tuple()"
		}
		keys = []string{s.Columns[0].Name}
	}

	return "(" + strings.Join(keys, ", ") + ")"
}
