package rockhopper

import (
	"database/sql"
	"fmt"
)

// PostgresDialect struct.
type PostgresDialect struct{}

func (d PostgresDialect) getTableNamesSQL() string {
	return `SELECT table_name FROM information_schema.tables 
		WHERE table_type = 'BASE TABLE' AND table_schema = 'public'`
}

func (d PostgresDialect) createVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            	id serial NOT NULL,
                version_id BIGINT NOT NULL,
                is_applied BOOLEAN NOT NULL,
                tstamp TIMESTAMP NULL DEFAULT NOW(),
                PRIMARY KEY(id)
            );`, tableName)
}

func (d PostgresDialect) insertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, version_id, is_applied) VALUES ($1, $2, $3);", tableName)
}

func (d PostgresDialect) selectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = $1", tableName)
}

func (d PostgresDialect) queryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = $1 ORDER BY id DESC", tableName)
}

func (d PostgresDialect) dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (d PostgresDialect) migrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=$1 ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (d PostgresDialect) deleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = $1 AND version_id = $2", tableName)
}
