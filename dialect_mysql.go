package rockhopper

import (
	"database/sql"
	"fmt"
)

// MySQLDialect struct.
type MySQLDialect struct{}

func (m MySQLDialect) getTableNamesSQL() string {
	return `SELECT table_name FROM information_schema.tables`
}

func (m MySQLDialect) createVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id SERIAL NOT NULL,
                package VARCHAR(125) NOT NULL DEFAULT 'main',
                version_id BIGINT NOT NULL,
                is_applied BOOLEAN NOT NULL,
                tstamp TIMESTAMP NULL DEFAULT NOW(),
                PRIMARY KEY(id),
				UNIQUE unique_version(version_id)
            );`, tableName)
}

func (m MySQLDialect) insertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, version_id, is_applied) VALUES (?, ?, ?);", tableName)
}

func (m MySQLDialect) dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied FROM %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m MySQLDialect) migrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=? ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (m MySQLDialect) deleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id = ?", tableName)
}
