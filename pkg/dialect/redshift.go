package dialect

// RedshiftDialect implements Dialect for Amazon Redshift. Redshift speaks a
// PostgreSQL-compatible wire protocol, so it embeds PostgresDialect and overrides
// only the introspection query, the "now" expression (sysdate) and the DDL.
type RedshiftDialect struct {
	PostgresDialect
}

// NewRedshiftDialect constructs a RedshiftDialect with its CRUD builder wired to
// its own tokens.
func NewRedshiftDialect() *RedshiftDialect {
	d := &RedshiftDialect{}
	d.CRUD = NewCRUD(d)
	return d
}

func (d *RedshiftDialect) NowExpr() string { return "sysdate" }

func (d *RedshiftDialect) TableNames() string {
	return "SELECT DISTINCT tablename FROM PG_TABLE_DEF WHERE schemaname = 'public'"
}

func (d *RedshiftDialect) CreateTable(s Schema) string { return buildCreateTable(redshiftDDL{}, s) }

func (d *RedshiftDialect) AddColumn(table string, c Column) (string, bool) {
	return buildAddColumn(redshiftDDL{}, table, c), true
}

// redshiftDDL renders Redshift DDL, differing from PostgreSQL in the identity
// column, the lack of a TEXT type, the sysdate default and no IF NOT EXISTS.
type redshiftDDL struct{ pgDDL }

func (redshiftDDL) sqlType(c Column) string {
	switch c.Type {
	case ColSerial:
		return "INTEGER NOT NULL identity(1, 1)"
	case ColText:
		return "VARCHAR(65535)"
	}
	return pgDDL{}.sqlType(c)
}

func (redshiftDDL) nowDefault() string { return "sysdate" }
func (redshiftDDL) ifNotExists() bool  { return false }
