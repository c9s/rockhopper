package dialect

import "fmt"

// MySQLDialect implements Dialect for MySQL.
type MySQLDialect struct {
	CRUD
}

// NewMySQLDialect constructs a MySQLDialect with its CRUD builder wired to its
// own tokens.
func NewMySQLDialect() *MySQLDialect {
	d := &MySQLDialect{}
	d.CRUD = NewCRUD(d)
	return d
}

func (d *MySQLDialect) Placeholder(int) string { return "?" }
func (d *MySQLDialect) NowExpr() string        { return "NOW()" }
func (d *MySQLDialect) TableNames() string     { return "SHOW TABLES" }

func (d *MySQLDialect) CreateTable(s Schema) string { return buildCreateTable(mysqlDDL{}, s) }

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
