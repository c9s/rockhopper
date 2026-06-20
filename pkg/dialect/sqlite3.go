package dialect

import "fmt"

// Sqlite3Dialect implements Dialect for SQLite3.
type Sqlite3Dialect struct {
	CRUD
}

// NewSqlite3Dialect constructs a Sqlite3Dialect with its CRUD builder wired to
// its own tokens.
func NewSqlite3Dialect() *Sqlite3Dialect {
	d := &Sqlite3Dialect{}
	d.CRUD = NewCRUD(d)
	return d
}

func (d *Sqlite3Dialect) Placeholder(int) string { return "?" }
func (d *Sqlite3Dialect) NowExpr() string        { return "datetime('now')" }
func (d *Sqlite3Dialect) TableNames() string {
	return "SELECT name FROM sqlite_master WHERE type='table'"
}

func (d *Sqlite3Dialect) CreateTable(s Schema) string { return buildCreateTable(sqliteDDL{}, s) }

// AddColumn is intentionally unsupported for SQLite: the legacy-table migration
// skips the ALTER on SQLite.
func (d *Sqlite3Dialect) AddColumn(string, Column) (string, bool) { return "", false }

// sqliteDDL renders SQLite DDL types.
type sqliteDDL struct{}

func (sqliteDDL) sqlType(c Column) string {
	switch c.Type {
	case ColSerial:
		return "INTEGER PRIMARY KEY AUTOINCREMENT"
	case ColBigInt:
		return "INTEGER"
	case ColBool:
		return "INTEGER"
	case ColVarchar:
		return fmt.Sprintf("VARCHAR(%d)", c.Size)
	case ColText:
		return "TEXT"
	case ColTimestamp:
		return "TIMESTAMP"
	}
	return ""
}

func (sqliteDDL) nowDefault() string { return "(datetime('now'))" }
func (sqliteDDL) ifNotExists() bool  { return true }
func (sqliteDDL) inlinePK() bool     { return true }
