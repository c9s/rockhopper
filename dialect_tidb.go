package rockhopper

import (
	"database/sql"
	"fmt"
)

// TiDBDialect struct.
type TiDBDialect struct{}

func (m TiDBDialect) getTableNamesSQL() string {
	return `SELECT table_name FROM information_schema.tables`
}

func (m TiDBDialect) createVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
                id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE,
                package VARCHAR(128) NOT NULL DEFAULT 'main',
            	source_file VARCHAR(255) NOT NULL DEFAULT '',
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp NULL default now(),
                PRIMARY KEY(id)
            );`, tableName)
}

func (m TiDBDialect) insertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES (?, ?, ?, ?)", tableName)
}

func (m TiDBDialect) selectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = ?", tableName)
}

func (m TiDBDialect) queryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = ? ORDER BY id DESC", tableName)
}

func (m TiDBDialect) dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m TiDBDialect) migrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE package = ? AND version_id = ? ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (m TiDBDialect) deleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = ? AND version_id = ?", tableName)
}
