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
            	id integer NOT NULL identity(1, 1),
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp NULL default sysdate,
                PRIMARY KEY(id)
            );`, tableName)
}

func (d RedshiftDialect) insertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (version_id, is_applied) VALUES ($1, $2);", tableName)
}

func (d RedshiftDialect) dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (d RedshiftDialect) migrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT tstamp, is_applied FROM %s WHERE version_id=$1 ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (d RedshiftDialect) deleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE version_id=$1;", tableName)
}
