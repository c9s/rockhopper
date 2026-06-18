package dialect

import (
	"database/sql"
	"fmt"
)

// TiDBDialect struct.
type TiDBDialect struct{}

func (m TiDBDialect) GetTableNamesSQL() string {
	return `SELECT table_name FROM information_schema.tables`
}

func (m TiDBDialect) CreateVersionTableSQL(tableName string) string {
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

func (m TiDBDialect) InsertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES (?, ?, ?, ?)", tableName)
}

func (m TiDBDialect) SelectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = ?", tableName)
}

func (m TiDBDialect) QueryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = ? ORDER BY id DESC", tableName)
}

func (m TiDBDialect) DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m TiDBDialect) MigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT id, tstamp, is_applied FROM %s WHERE package = ? AND version_id = ? ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (m TiDBDialect) DeleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = ? AND version_id = ?", tableName)
}

func (m TiDBDialect) CreateDataMigrationTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
                id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE,
                package VARCHAR(128) NOT NULL DEFAULT 'main',
                version_id BIGINT NOT NULL,
                name VARCHAR(255) NOT NULL DEFAULT '',
                status VARCHAR(32) NOT NULL DEFAULT 'pending',
                checkpoint TEXT,
                lease_owner VARCHAR(255),
                lease_expires_at BIGINT NOT NULL DEFAULT 0,
                created_at TIMESTAMP NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
                PRIMARY KEY(id),
                UNIQUE KEY uniq_data_migration (package, version_id)
            );`, tableName)
}

func (m TiDBDialect) InsertDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, version_id, name, status, checkpoint) VALUES (?, ?, ?, ?, ?)", tableName)
}

func (m TiDBDialect) SelectDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT status, checkpoint FROM %s WHERE package = ? AND version_id = ?", tableName)
}

func (m TiDBDialect) AcquireDataMigrationLeaseSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET lease_owner = ?, lease_expires_at = ?, updated_at = NOW() "+
		"WHERE package = ? AND version_id = ? AND (lease_owner IS NULL OR lease_owner = ? OR lease_expires_at < ?)", tableName)
}

func (m TiDBDialect) CommitDataBatchSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET status = ?, checkpoint = ?, lease_expires_at = ?, updated_at = NOW() "+
		"WHERE package = ? AND version_id = ? AND lease_owner = ?", tableName)
}

func (m TiDBDialect) ReleaseDataMigrationLeaseSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET status = ?, lease_owner = NULL, lease_expires_at = 0, updated_at = NOW() "+
		"WHERE package = ? AND version_id = ? AND lease_owner = ?", tableName)
}
