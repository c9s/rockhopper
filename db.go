package rockhopper

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type DB struct {
	*sql.DB

	driverName string
	dialect    SQLDialect
	tableName  string
}

func OpenByConfig(config *Config) (*DB, error) {
	dialectName := config.Dialect
	if len(dialectName) == 0 {
		dialectName = config.Driver
	}

	dialect, err := LoadDialect(dialectName)
	if err != nil {
		return nil, err
	}

	return Open(config.Driver, config.DSN, dialect)
}

// Open creates a connection to a database
func Open(driverName string, dsn string, dialect SQLDialect) (*DB, error) {
	switch driverName {
	case "mssql":
		driverName = "sqlserver"
	case "redshift":
		driverName = "postgres"
	case "tidb":
		driverName = "mysql"
	}

	switch driverName {
	// supported drivers
	case "postgres", "sqlite3", "mysql", "sqlserver":
	default:
		return nil, fmt.Errorf("unsupported driver %s", driverName)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	return &DB{
		dialect:    dialect,
		driverName: driverName,
		DB:         db,
		tableName:  defaultTableName,
	}, nil
}

func (db *DB) deleteVersion(ctx context.Context, tx SQLExecutor, version int64) error {
	if _, err := tx.ExecContext(ctx, db.dialect.deleteVersionSQL(db.tableName), version); err != nil {
		return errors.Wrap(err, "failed to delete migration record")
	}

	return nil
}

func (db *DB) insertVersion(ctx context.Context, tx SQLExecutor, version int64) error {
	if _, err := tx.ExecContext(ctx, db.dialect.insertVersionSQL(db.tableName), version, true); err != nil {
		return errors.Wrap(err, "failed to insert new migration record")
	}

	return nil
}

func (db *DB) FindMigration(version int64) (*MigrationRecord, error) {
	var row MigrationRecord

	var q = db.dialect.migrationSQL(db.tableName)
	var err = db.QueryRow(q, version).Scan(&row.Time, &row.IsApplied)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, errors.Wrap(err, "failed to query the latest migration")
		}
	}

	return &row, nil

}

func (db *DB) LoadMigrationRecords() ([]MigrationRecord, error) {
	rows, err := db.dialect.dbVersionQuery(db.DB, db.tableName)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.WithError(err).Error("row close error")
		}
	}()

	// The most recent record for each migration specifies
	// whether it has been applied or rolled back.
	// The first version we find that has been applied is the current version.
	// The rows are in descending order.
	var records []MigrationRecord
	for rows.Next() {
		var row MigrationRecord
		if err = rows.Scan(&row.VersionID, &row.IsApplied); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}

		records = append(records, row)
	}

	if err := rows.Err(); err != nil {
		return records, errors.Wrap(err, "failed to read the next row")
	}

	return records, nil
}

func (db *DB) CurrentVersion() (int64, error) {
	rows, err := db.dialect.dbVersionQuery(db.DB, db.tableName)
	if err != nil {
		return 0, db.createVersionTable()
	}

	if err := rows.Close(); err != nil {
		return 0, err
	}

	records, err := db.LoadMigrationRecords()
	if err != nil {
		return 0, err
	}

	// The most recent record for each migration specifies
	// whether it has been applied or rolled back.
	// The first version we find that has been applied is the current version.
	toSkip := make([]int64, 0)
	for _, row := range records {
		// have we already marked this version to be skipped?
		skip := false
		for _, v := range toSkip {
			if v == row.VersionID {
				skip = true
				break
			}
		}

		if skip {
			continue
		}

		// if version has been applied we're done
		if row.IsApplied {
			return row.VersionID, nil
		}

		// latest version of migration has not been applied.
		toSkip = append(toSkip, row.VersionID)
	}

	return 0, ErrNoCurrentVersion
}

// Create the db version table
// and insert the initial 0 value into it
func (db *DB) createVersionTable() error {
	txn, err := db.Begin()
	if err != nil {
		return err
	}

	if _, err := txn.Exec(db.dialect.createVersionTableSQL(db.tableName)); err != nil {
		if err := txn.Rollback(); err != nil {
			log.WithError(err).Error("create version table, rollback error")
		}
		return err
	}

	version := 0
	applied := true
	if _, err := txn.Exec(db.dialect.insertVersionSQL(db.tableName), version, applied); err != nil {
		if err := txn.Rollback(); err != nil {
			log.WithError(err).Error("insert version, rollback error")
		}

		return err
	}

	return txn.Commit()
}
