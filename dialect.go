package rockhopper

import (
	"database/sql"
	"fmt"

	"github.com/c9s/rockhopper/v2/pkg/dialect"
)

const (
	DialectPostgres = "postgres"
	DialectMySQL    = "mysql"
	DialectSQLite3  = "sqlite3"
	DialectRedshift = "redshift"
	DialectTiDB     = "tidb"
)

// SQLDialect abstracts the details of specific SQL dialects
// for goose's few SQL specific statements
type SQLDialect interface {
	GetTableNamesSQL() string                      // return the sql string to get the table names
	CreateVersionTableSQL(tableName string) string // sql string to create the db version table
	InsertVersionSQL(tableName string) string      // sql string to insert the initial version table row
	DeleteVersionSQL(tableName string) string      // sql string to delete version

	// MigrationSQL returns the sql string to retrieve migrations
	MigrationSQL(tableName string) string

	// SelectLastVersionSQL returns the sql string to get the latest version
	SelectLastVersionSQL(tableName string) string

	// QueryVersionsSQL returns the sql string to query the version info descending
	QueryVersionsSQL(tableName string) string
	DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error)
}

func LoadDialect(d string) (SQLDialect, error) {
	switch d {
	case DialectPostgres:
		return &dialect.PostgresDialect{}, nil
	case DialectMySQL:
		return &dialect.MySQLDialect{}, nil
	case DialectSQLite3:
		return &dialect.Sqlite3Dialect{}, nil
	case DialectRedshift:
		return &dialect.RedshiftDialect{}, nil
	case DialectTiDB:
		return &dialect.TiDBDialect{}, nil
	}

	return nil, fmt.Errorf("%q: unknown dialect", d)
}
