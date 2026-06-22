package dialect

// Dialect abstracts the per-database SQL details rockhopper needs. The simple
// single-table CRUD/lease statements come from the embedded Builder (rendered
// generically from the dialect's Tokens), while the genuinely divergent parts —
// DDL and introspection — are implemented per dialect.
type Dialect interface {
	Builder

	// CreateTable renders the CREATE TABLE statement for a schema.
	CreateTable(s Schema) string

	// TableNames returns the introspection query listing existing table names.
	TableNames() string

	// AddColumn renders an ALTER TABLE ... ADD COLUMN statement. supported is
	// false for dialects where rockhopper intentionally skips the alter (SQLite).
	AddColumn(table string, c Column) (sql string, supported bool)
}
