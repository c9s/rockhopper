package dialect

import "fmt"

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct {
	CRUD
}

// NewPostgresDialect constructs a PostgresDialect with its CRUD builder wired to
// its own tokens.
func NewPostgresDialect() *PostgresDialect {
	d := &PostgresDialect{}
	d.CRUD = NewCRUD(d)
	return d
}

func (d *PostgresDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (d *PostgresDialect) NowExpr() string          { return "NOW()" }

func (d *PostgresDialect) TableNames() string {
	return "SELECT table_name FROM information_schema.tables\n" +
		"\t\tWHERE table_type = 'BASE TABLE' AND table_schema = 'public'"
}

func (d *PostgresDialect) CreateTable(s Schema) string { return buildCreateTable(pgDDL{}, s) }

func (d *PostgresDialect) AddColumn(table string, c Column) (string, bool) {
	return buildAddColumn(pgDDL{}, table, c), true
}

// pgDDL renders PostgreSQL DDL types.
type pgDDL struct{}

func (pgDDL) sqlType(c Column) string {
	switch c.Type {
	case ColSerial:
		return "serial NOT NULL"
	case ColBigInt:
		return "BIGINT"
	case ColBool:
		return "BOOLEAN"
	case ColVarchar:
		return fmt.Sprintf("VARCHAR(%d)", c.Size)
	case ColText:
		return "TEXT"
	case ColTimestamp:
		return "TIMESTAMP"
	}
	return ""
}

func (pgDDL) nowDefault() string { return "NOW()" }
func (pgDDL) ifNotExists() bool  { return true }
func (pgDDL) inlinePK() bool     { return false }
