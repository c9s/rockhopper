package rockhopper

import (
	"database/sql"
	"fmt"
)

var tableName = "goose_db_version"

// TableName returns goose db version table name
func TableName() string {
	return tableName
}

// SetTableName set goose db version table name
func SetTableName(n string) {
	tableName = n
}

// SQLDialect abstracts the details of specific SQL dialects
// for goose's few SQL specific statements
type SQLDialect interface {
	createVersionTableSQL() string // sql string to create the db version table
	insertVersionSQL() string      // sql string to insert the initial version table row
	deleteVersionSQL() string      // sql string to delete version
	migrationSQL() string          // sql string to retrieve migrations
	dbVersionQuery(db *sql.DB) (*sql.Rows, error)
}

var dialect SQLDialect = &PostgresDialect{}

// GetDialect gets the SQLDialect
func GetDialect() SQLDialect {
	return dialect
}

// SetDialect sets the SQLDialect
func SetDialect(d string) error {
	switch d {
	case "postgres":
		dialect = &PostgresDialect{}
	case "mysql":
		dialect = &MySQLDialect{}
	case "sqlite3":
		dialect = &Sqlite3Dialect{}
	case "mssql":
		dialect = &SqlServerDialect{}
	case "redshift":
		dialect = &RedshiftDialect{}
	case "tidb":
		dialect = &TiDBDialect{}
	default:
		return fmt.Errorf("%q: unknown dialect", d)
	}

	return nil
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

////////////////////////////
// Postgres
////////////////////////////

// PostgresDialect struct.
type PostgresDialect struct{}

func (d PostgresDialect) createVersionTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            	id serial NOT NULL,
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp NULL default now(),
                PRIMARY KEY(id)
            );`, TableName())
}

func (d PostgresDialect) insertVersionSQL() string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES ($1, $2);", TableName())
}

func (d PostgresDialect) dbVersionQuery(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied from %s ORDER BY id DESC", TableName()))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (d PostgresDialect) migrationSQL() string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=$1 ORDER BY tstamp DESC LIMIT 1", TableName())
}

func (d PostgresDialect) deleteVersionSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=$1;", TableName())
}

////////////////////////////
// MySQL
////////////////////////////

// MySQLDialect struct.
type MySQLDialect struct{}

func (m MySQLDialect) createVersionTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id serial NOT NULL,
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp NULL default now(),
                PRIMARY KEY(id)
            );`, TableName())
}

func (m MySQLDialect) insertVersionSQL() string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES (?, ?);", TableName())
}

func (m MySQLDialect) dbVersionQuery(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied FROM %s ORDER BY id DESC", TableName()))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m MySQLDialect) migrationSQL() string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=? ORDER BY tstamp DESC LIMIT 1", TableName())
}

func (m MySQLDialect) deleteVersionSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=?;", TableName())
}

////////////////////////////
// MSSQL
////////////////////////////

// SqlServerDialect struct.
type SqlServerDialect struct{}

func (m SqlServerDialect) createVersionTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE %s (
                id INT NOT NULL IDENTITY(1,1) PRIMARY KEY,
                version_id BIGINT NOT NULL,
                is_applied BIT NOT NULL,
                tstamp DATETIME NULL DEFAULT CURRENT_TIMESTAMP
            );`, TableName())
}

func (m SqlServerDialect) insertVersionSQL() string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES (@p1, @p2);", TableName())
}

func (m SqlServerDialect) dbVersionQuery(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied FROM %s ORDER BY id DESC", TableName()))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m SqlServerDialect) migrationSQL() string {
	const tpl = `
WITH Migrations AS
(
    SELECT tstamp, is_applied,
    ROW_NUMBER() OVER (ORDER BY tstamp) AS 'RowNumber'
    FROM %s 
	WHERE version_id=@p1
) 
SELECT tstamp, is_applied 
FROM Migrations 
WHERE RowNumber BETWEEN 1 AND 2
ORDER BY tstamp DESC
`
	return fmt.Sprintf(tpl, TableName())
}

func (m SqlServerDialect) deleteVersionSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=@p1;", TableName())
}

////////////////////////////
// sqlite3
////////////////////////////

// Sqlite3Dialect struct.
type Sqlite3Dialect struct{}

func (m Sqlite3Dialect) createVersionTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                version_id INTEGER NOT NULL,
                is_applied INTEGER NOT NULL,
                tstamp TIMESTAMP DEFAULT (datetime('now'))
            );`, TableName())
}

func (m Sqlite3Dialect) insertVersionSQL() string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES (?, ?);", TableName())
}

func (m Sqlite3Dialect) dbVersionQuery(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied from %s ORDER BY id DESC", TableName()))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m Sqlite3Dialect) migrationSQL() string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=? ORDER BY tstamp DESC LIMIT 1", TableName())
}

func (m Sqlite3Dialect) deleteVersionSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=?;", TableName())
}

////////////////////////////
// Redshift
////////////////////////////

// RedshiftDialect struct.
type RedshiftDialect struct{}

func (rs RedshiftDialect) createVersionTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE %s (
            	id integer NOT NULL identity(1, 1),
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp NULL default sysdate,
                PRIMARY KEY(id)
            );`, TableName())
}

func (rs RedshiftDialect) insertVersionSQL() string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES ($1, $2);", TableName())
}

func (rs RedshiftDialect) dbVersionQuery(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied from %s ORDER BY id DESC", TableName()))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m RedshiftDialect) migrationSQL() string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=$1 ORDER BY tstamp DESC LIMIT 1", TableName())
}

func (rs RedshiftDialect) deleteVersionSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=$1;", TableName())
}

////////////////////////////
// TiDB
////////////////////////////

// TiDBDialect struct.
type TiDBDialect struct{}

func (m TiDBDialect) createVersionTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE %s (
                id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE,
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp NULL default now(),
                PRIMARY KEY(id)
            );`, TableName())
}

func (m TiDBDialect) insertVersionSQL() string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES (?, ?);", TableName())
}

func (m TiDBDialect) dbVersionQuery(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied from %s ORDER BY id DESC", TableName()))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m TiDBDialect) migrationSQL() string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=? ORDER BY tstamp DESC LIMIT 1", TableName())
}

func (m TiDBDialect) deleteVersionSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=?;", TableName())
}
