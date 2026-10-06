package dialect

import "fmt"

// MySQLDialect implements Dialect for MySQL.
type MySQLDialect struct {
	LeaseCRUD
}

// NewMySQLDialect constructs a MySQLDialect with its CRUD builder wired to its
// own tokens.
func NewMySQLDialect() *MySQLDialect {
	d := &MySQLDialect{}
	d.LeaseCRUD = NewLeaseCRUD(d)
	return d
}

// Placeholder returns MySQL's question-mark bind marker.
func (d *MySQLDialect) Placeholder(int) string { return "?" }

// NowExpr returns MySQL's current-time expression for DML.
func (d *MySQLDialect) NowExpr() string { return "NOW()" }

// TableNames returns the query used to list tables in the current database.
func (d *MySQLDialect) TableNames() string { return "SHOW TABLES" }

// CreateTable renders a MySQL CREATE TABLE statement for s.
func (d *MySQLDialect) CreateTable(s Schema) string { return buildCreateTable(mysqlDDL{}, s) }

// AddColumn renders a MySQL ALTER TABLE statement for c.
func (d *MySQLDialect) AddColumn(table string, c Column) (string, bool) {
	return buildAddColumn(mysqlDDL{}, table, c), true
}

// mysqlDDL renders MySQL DDL types.
type mysqlDDL struct{}

func (mysqlDDL) sqlType(c Column) string {
	switch c.Type {
	case ColSerial:
		return "SERIAL NOT NULL"
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

func (mysqlDDL) nowDefault() string { return "NOW()" }
func (mysqlDDL) ifNotExists() bool  { return true }
func (mysqlDDL) inlinePK() bool     { return false }
