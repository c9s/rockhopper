package dialect

// TiDBDialect implements Dialect for TiDB. TiDB is MySQL wire-compatible, so it
// embeds MySQLDialect and overrides only the introspection query and the DDL.
type TiDBDialect struct {
	MySQLDialect
}

// NewTiDBDialect constructs a TiDBDialect with its CRUD builder wired to its own
// tokens (so dialect-specific overrides dispatch correctly).
func NewTiDBDialect() *TiDBDialect {
	d := &TiDBDialect{}
	d.LeaseCRUD = NewLeaseCRUD(d)
	return d
}

func (d *TiDBDialect) TableNames() string {
	return "SELECT table_name FROM information_schema.tables"
}

func (d *TiDBDialect) CreateTable(s Schema) string { return buildCreateTable(tidbDDL{}, s) }

func (d *TiDBDialect) AddColumn(table string, c Column) (string, bool) {
	return buildAddColumn(tidbDDL{}, table, c), true
}

// tidbDDL renders TiDB DDL, differing from MySQL in the auto-increment column
// and the absence of CREATE TABLE IF NOT EXISTS.
type tidbDDL struct{ mysqlDDL }

func (tidbDDL) sqlType(c Column) string {
	if c.Type == ColSerial {
		return "BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE"
	}
	return mysqlDDL{}.sqlType(c)
}

func (tidbDDL) ifNotExists() bool { return false }
