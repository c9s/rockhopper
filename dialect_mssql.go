package rockhopper

import (
	"database/sql"
	"fmt"
)

// SqlServerDialect struct.
type SqlServerDialect struct{}

func (m SqlServerDialect) getTableNamesSQL() string {
	return `SELECT * FROM SYS.TABLES;`
}

func (m SqlServerDialect) createVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
                id INT NOT NULL IDENTITY(1,1) PRIMARY KEY,
    			package VARCHAR(128) NOT NULL DEFAULT 'main',
    			source_file VARCAHR(255) NOT NULL DEFAULT '',
                version_id BIGINT NOT NULL,
                is_applied BIT NOT NULL,
                tstamp DATETIME NULL DEFAULT CURRENT_TIMESTAMP
            );`, tableName)
}

func (m SqlServerDialect) insertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES (@p1, @p2, @p3, @p4)", tableName)
}

func (m SqlServerDialect) selectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = @p1", tableName)
}

func (m SqlServerDialect) queryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = @p1 ORDER BY id DESC", tableName)
}

func (m SqlServerDialect) dbVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT package, version_id, is_applied FROM %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m SqlServerDialect) migrationSQL(tableName string) string {
	const tpl = `
WITH Migrations AS
(
    SELECT id, tstamp, is_applied,
    ROW_NUMBER() OVER (ORDER BY tstamp) AS 'RowNumber'
    FROM %s 
	WHERE package = @p1 AND version_id = @p2
) 
SELECT id, tstamp, is_applied 
FROM Migrations 
WHERE RowNumber BETWEEN 1 AND 2
ORDER BY tstamp DESC
`
	return fmt.Sprintf(tpl, tableName)
}

func (m SqlServerDialect) deleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = @p1 AND version_id = @p2", tableName)
}
