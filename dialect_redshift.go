package rockhopper

import (
	"database/sql"
	"fmt"
)

// RedshiftDialect struct.
type RedshiftDialect struct{}

func (d RedshiftDialect) getTableNamesSQL() string {
	return `SELECT DISTINCT tablename FROM PG_TABLE_DEF WHERE schemaname = 'public'`
}

func (d RedshiftDialect) createVersionTableSQL(tableName string) string {
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

func (d RedshiftDialect) insertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES ($1, $2, $3, $4)", tableName)
}

func (d RedshiftDialect) selectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = $1", tableName)
}

func (d RedshiftDialect) queryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = $1 ORDER BY id DESC", tableName)
}

func (d RedshiftDialect) dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (d RedshiftDialect) migrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT id, tstamp, is_applied FROM %s WHERE package = $1 AND version_id = $2 ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (d RedshiftDialect) deleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = $1 AND version_id = $2", tableName)
}
