package dialect

import (
	"database/sql"
	"fmt"
)

// PostgresDialect struct.
type PostgresDialect struct{}

func (d PostgresDialect) GetTableNamesSQL() string {
	return `SELECT table_name FROM information_schema.tables
		WHERE table_type = 'BASE TABLE' AND table_schema = 'public'`
}

func (d PostgresDialect) CreateVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            	id serial NOT NULL,
            	package VARCHAR(128) NOT NULL DEFAULT 'main',
            	source_file VARCHAR(255) NOT NULL DEFAULT '',
                version_id BIGINT NOT NULL,
                is_applied BOOLEAN NOT NULL,
                tstamp TIMESTAMP NULL DEFAULT NOW(),
                PRIMARY KEY(id)
            );`, tableName)
}

func (d PostgresDialect) InsertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES ($1, $2, $3, $4)", tableName)
}

func (d PostgresDialect) SelectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = $1", tableName)
}

func (d PostgresDialect) QueryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = $1 ORDER BY id DESC", tableName)
}

func (d PostgresDialect) DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (d PostgresDialect) MigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT id, tstamp, is_applied FROM %s WHERE package = $1 AND version_id = $2 ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (d PostgresDialect) DeleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = $1 AND version_id = $2", tableName)
}

func (d PostgresDialect) CreateDataMigrationTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id serial NOT NULL,
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
                UNIQUE(package, version_id)
            );`, tableName)
}

func (d PostgresDialect) InsertDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, version_id, name, status, checkpoint) VALUES ($1, $2, $3, $4, $5)", tableName)
}

func (d PostgresDialect) SelectDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT status, checkpoint FROM %s WHERE package = $1 AND version_id = $2", tableName)
}

func (d PostgresDialect) AcquireDataMigrationLeaseSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET lease_owner = $1, lease_expires_at = $2, updated_at = NOW() "+
		"WHERE package = $3 AND version_id = $4 AND (lease_owner IS NULL OR lease_owner = $5 OR lease_expires_at < $6)", tableName)
}

func (d PostgresDialect) CommitDataBatchSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET status = $1, checkpoint = $2, lease_expires_at = $3, updated_at = NOW() "+
		"WHERE package = $4 AND version_id = $5 AND lease_owner = $6", tableName)
}

func (d PostgresDialect) ReleaseDataMigrationLeaseSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET status = $1, lease_owner = NULL, lease_expires_at = 0, updated_at = NOW() "+
		"WHERE package = $2 AND version_id = $3 AND lease_owner = $4", tableName)
}
