package dialect

import (
	"database/sql"
	"fmt"
)

// MySQLDialect struct.
type MySQLDialect struct{}

func (m MySQLDialect) GetTableNamesSQL() string {
	return `SHOW TABLES`
}

func (m MySQLDialect) CreateVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id SERIAL NOT NULL,
                package VARCHAR(125) NOT NULL DEFAULT 'main',
    			source_file VARCHAR(255) NOT NULL DEFAULT '',
                version_id BIGINT NOT NULL,
                is_applied BOOLEAN NOT NULL,
                tstamp TIMESTAMP NOT NULL DEFAULT NOW(),
                PRIMARY KEY(id),
				UNIQUE unique_version(version_id)
            );`, tableName)
}

func (m MySQLDialect) InsertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES (?, ?, ?, ?)", tableName)
}

func (m MySQLDialect) SelectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = ?", tableName)
}

func (m MySQLDialect) QueryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = ? ORDER BY id DESC", tableName)
}

func (m MySQLDialect) DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied FROM %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m MySQLDialect) MigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT id, tstamp, is_applied FROM %s WHERE package = ? AND version_id = ? ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (m MySQLDialect) DeleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = ? AND version_id = ?", tableName)
}

func (m MySQLDialect) CreateDataMigrationTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id SERIAL NOT NULL,
                package VARCHAR(125) NOT NULL DEFAULT 'main',
                version_id BIGINT NOT NULL,
                name VARCHAR(255) NOT NULL DEFAULT '',
                status VARCHAR(32) NOT NULL DEFAULT 'pending',
                checkpoint TEXT,
                created_at TIMESTAMP NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
                PRIMARY KEY(id),
                UNIQUE KEY uniq_data_migration (package, version_id)
            );`, tableName)
}

func (m MySQLDialect) InsertDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, version_id, name, status, checkpoint) VALUES (?, ?, ?, ?, ?)", tableName)
}

func (m MySQLDialect) UpdateDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET status = ?, checkpoint = ?, updated_at = NOW() WHERE package = ? AND version_id = ?", tableName)
}

func (m MySQLDialect) SelectDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT status, checkpoint FROM %s WHERE package = ? AND version_id = ?", tableName)
}
