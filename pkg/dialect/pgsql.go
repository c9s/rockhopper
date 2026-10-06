package dialect

import "fmt"

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct {
	LeaseCRUD
}

// NewPostgresDialect constructs a PostgresDialect with its CRUD builder wired to
// its own tokens.
func NewPostgresDialect() *PostgresDialect {
	d := &PostgresDialect{}
	d.LeaseCRUD = NewLeaseCRUD(d)
	return d
}

// Placeholder returns the numbered bind marker for n.
func (d *PostgresDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }

// NowExpr returns PostgreSQL's current-time expression for DML.
func (d *PostgresDialect) NowExpr() string { return sqlNowExpression }

// TableNames returns the query used to list tables in the public schema.
func (d *PostgresDialect) TableNames() string {
	return "SELECT table_name FROM information_schema.tables\n" +
		"\t\tWHERE table_type = 'BASE TABLE' AND table_schema = 'public'"
}

// CreateTable renders a PostgreSQL CREATE TABLE statement for s.
func (d *PostgresDialect) CreateTable(s Schema) string { return buildCreateTable(pgDDL{}, s) }

// AddColumn renders a PostgreSQL ALTER TABLE statement for c.
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
		return sqlTextType
	case ColTimestamp:
		return sqlTimestampType
	}
	return ""
}

func (pgDDL) nowDefault() string { return sqlNowExpression }
func (pgDDL) ifNotExists() bool  { return true }
func (pgDDL) inlinePK() bool     { return false }
