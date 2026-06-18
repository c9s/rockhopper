package dialect

import (
	"database/sql"
	"fmt"
)

// RedshiftDialect struct.
type RedshiftDialect struct{}

func (d RedshiftDialect) GetTableNamesSQL() string {
	return `SELECT DISTINCT tablename FROM PG_TABLE_DEF WHERE schemaname = 'public'`
}

func (d RedshiftDialect) CreateVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
            	id INTEGER NOT NULL identity(1, 1),
                package VARCHAR(128) NOT NULL DEFAULT 'main',
            	source_file VARCHAR(255) NOT NULL DEFAULT '',
                version_id BIGINT NOT NULL,
                is_applied BOOLEAN NOT NULL,
                tstamp TIMESTAMP NULL default sysdate,
                PRIMARY KEY(id)
            );`, tableName)
}

func (d RedshiftDialect) InsertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES ($1, $2, $3, $4)", tableName)
}

func (d RedshiftDialect) SelectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = $1", tableName)
}

func (d RedshiftDialect) QueryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = $1 ORDER BY id DESC", tableName)
}

func (d RedshiftDialect) DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (d RedshiftDialect) MigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT id, tstamp, is_applied FROM %s WHERE package = $1 AND version_id = $2 ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (d RedshiftDialect) DeleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = $1 AND version_id = $2", tableName)
}

func (d RedshiftDialect) CreateDataMigrationTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
                id INTEGER NOT NULL identity(1, 1),
                package VARCHAR(128) NOT NULL DEFAULT 'main',
                version_id BIGINT NOT NULL,
                name VARCHAR(255) NOT NULL DEFAULT '',
                status VARCHAR(32) NOT NULL DEFAULT 'pending',
                checkpoint VARCHAR(65535),
                created_at TIMESTAMP NULL default sysdate,
                updated_at TIMESTAMP NULL default sysdate,
                PRIMARY KEY(id)
            );`, tableName)
}

func (d RedshiftDialect) InsertDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, version_id, name, status, checkpoint) VALUES ($1, $2, $3, $4, $5)", tableName)
}

func (d RedshiftDialect) UpdateDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("UPDATE %s SET status = $1, checkpoint = $2, updated_at = sysdate WHERE package = $3 AND version_id = $4", tableName)
}

func (d RedshiftDialect) SelectDataMigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT status, checkpoint FROM %s WHERE package = $1 AND version_id = $2", tableName)
}
