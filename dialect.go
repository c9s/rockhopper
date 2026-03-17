package rockhopper

import (
	"database/sql"
	"fmt"
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
	getTableNamesSQL() string                      // return the sql string to get the table names
	createVersionTableSQL(tableName string) string // sql string to create the db version table
	insertVersionSQL(tableName string) string      // sql string to insert the initial version table row
	deleteVersionSQL(tableName string) string      // sql string to delete version

	// migrationSQL returns the sql string to retrieve migrations
	migrationSQL(tableName string) string

	// selectLastVersionSQL returns the sql string to get the latest version
	selectLastVersionSQL(tableName string) string

	// queryVersionsSQL returns the sql string to query the version info descending
	queryVersionsSQL(tableName string) string
	dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error)
}

func LoadDialect(d string) (SQLDialect, error) {
	switch d {
	case DialectPostgres:
		return &PostgresDialect{}, nil
	case DialectMySQL:
		return &MySQLDialect{}, nil
	case DialectSQLite3:
		return &Sqlite3Dialect{}, nil
	case DialectRedshift:
		return &RedshiftDialect{}, nil
	case DialectTiDB:
		return &TiDBDialect{}, nil
	}

	return nil, fmt.Errorf("%q: unknown dialect", d)
}
