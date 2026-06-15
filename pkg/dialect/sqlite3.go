package dialect

import (
	"database/sql"
	"fmt"
)

// Sqlite3Dialect struct.
type Sqlite3Dialect struct{}

func (m Sqlite3Dialect) GetTableNamesSQL() string {
	return `SELECT name FROM sqlite_master WHERE type='table'`
}

func (m Sqlite3Dialect) CreateVersionTableSQL(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                package TEXT NOT NULL DEFAULT 'main',
            	source_file VARCHAR(255) NOT NULL DEFAULT '',
                version_id INTEGER NOT NULL,
                is_applied INTEGER NOT NULL,
                tstamp TIMESTAMP DEFAULT (datetime('now'))
            );`, tableName)
}

func (m Sqlite3Dialect) InsertVersionSQL(tableName string) string {
	return fmt.Sprintf("INSERT INTO %s (package, source_file, version_id, is_applied) VALUES (?, ?, ?, ?)", tableName)
}

func (m Sqlite3Dialect) SelectLastVersionSQL(tableName string) string {
	return fmt.Sprintf("SELECT MAX(version_id) FROM %s WHERE package = ?", tableName)
}

func (m Sqlite3Dialect) QueryVersionsSQL(tableName string) string {
	return fmt.Sprintf("SELECT package, version_id, is_applied, tstamp FROM %s WHERE package = ? ORDER BY id DESC", tableName)
}

func (m Sqlite3Dialect) DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT id, package, version_id, is_applied from %s ORDER BY id DESC", tableName))
	if err != nil {
		return nil, err
	}

	return rows, err
}

func (m Sqlite3Dialect) MigrationSQL(tableName string) string {
	return fmt.Sprintf("SELECT id, tstamp, is_applied FROM %s WHERE package = ? AND version_id = ? ORDER BY tstamp DESC LIMIT 1", tableName)
}

func (m Sqlite3Dialect) DeleteVersionSQL(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE package = ? AND version_id = ?", tableName)
}
