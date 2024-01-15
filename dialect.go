package rockhopper

import (
	"database/sql"
	"fmt"
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
	case "postgres":
		return &PostgresDialect{}, nil
	case "mysql":
		return &MySQLDialect{}, nil
	case "sqlite3":
		return &Sqlite3Dialect{}, nil
	case "mssql":
		return &SqlServerDialect{}, nil
	case "redshift":
		return &RedshiftDialect{}, nil
	case "tidb":
		return &TiDBDialect{}, nil
	}

	return nil, fmt.Errorf("%q: unknown dialect", d)
}
