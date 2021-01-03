package rockhopper

import (
	"database/sql"
	"fmt"
)

type DB struct {
	*sql.DB

	driverName string
	dialect SQLDialect
}

// Open creates a connection to a database
func Open(driverName string, dbstring string, dialect SQLDialect) (*DB, error) {
	switch driverName {
	case "mssql":
		driverName = "sqlserver"
	case "redshift":
		driverName = "postgres"
	case "tidb":
		driverName = "mysql"
	}

	switch driverName {
	// supported drivers
	case "postgres", "sqlite3", "mysql", "sqlserver":
	default:
		return nil, fmt.Errorf("unsupported driver %s", driverName)
	}

	db, err := sql.Open(driverName, dbstring)
	if err != nil {
		return nil, err
	}

	return &DB{
		dialect: dialect,
		driverName: driverName,
		DB: db,
	}, nil
}
