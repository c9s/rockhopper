package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	VersionGoose        = 0
	VersionRockhopperV1 = 1
)

// legacyGooseTableName is the legacy table name
const legacyGooseTableName = "goose_db_version"

// TableName is the migration version table name
const TableName = "rockhopper_versions"

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type DB struct {
	*sql.DB

	driverName string
	dialect    SQLDialect
	tableName  string
}

func OpenWithConfig(config *Config) (*DB, error) {
	dialectName := config.Dialect
	if len(dialectName) == 0 {
		dialectName = config.Driver
	}

	dialect, err := LoadDialect(dialectName)
	if err != nil {
		return nil, err
	}

	dsn := config.DSN
	if len(dsn) == 0 {
		dsn, err = BuildDSNFromEnvVars(config.Driver)
		if err != nil {
			return nil, errors.Wrap(err, "dsn is not defined, can not build dsn")
		}
	}

	return Open(config.Driver, dialect, dsn, TableName)
}

func BuildDSNFromEnvVars(driver string) (string, error) {
	switch driver {
	case "mysql":
		return buildMySqlDSN()

	}
	return "", fmt.Errorf("can not build dsn for driver %s", driver)
}

func castDriverName(driver string) string {
	switch driver {
	case "mssql":
		return "sqlserver"
	case "redshift":
		return "postgres"
	case "tidb":
		return "mysql"
	}

	return driver
}

func OpenWithEnv(prefix string) (*DB, error) {
	driverName := os.Getenv(prefix + "_DRIVER")
	if driverName == "" {
		return nil, fmt.Errorf("env %s_DRIVER is not defined", prefix)
	}

	dialectName := os.Getenv(prefix + "_DIALECT")
	if dialectName == "" {
		dialectName = driverName
	}

	dialect, err := LoadDialect(dialectName)
	if err != nil {
		return nil, err
	}

	dsn := os.Getenv(prefix + "_DSN")
	return Open(driverName, dialect, dsn, TableName)
}

// Open creates a connection to a database
func Open(driverName string, dialect SQLDialect, dsn string, tableName string) (*DB, error) {
	driverName = castDriverName(driverName)

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

	return New(driverName, dialect, db, tableName), nil
}

func New(driverName string, dialect SQLDialect, db *sql.DB, tableName string) *DB {
	return &DB{
		dialect:    dialect,
		driverName: driverName,
		DB:         db,
		tableName:  tableName,
	}
}

func (db *DB) deleteVersion(ctx context.Context, tx SQLExecutor, pkgName string, version int64) error {
	if _, err := tx.ExecContext(ctx, db.dialect.deleteVersionSQL(db.tableName), pkgName, version); err != nil {
		return errors.Wrap(err, "failed to delete migration record")
	}

	return nil
}

func (db *DB) getTableNames(ctx context.Context) ([]string, error) {
	q := db.dialect.getTableNamesSQL()
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return tableNames, err
		}

		tableNames = append(tableNames, tableName)
	}

	return tableNames, nil
}

func (db *DB) insertVersion(ctx context.Context, tx SQLExecutor, pkgName string, version int64, applied bool) error {
	if _, err := tx.ExecContext(ctx, db.dialect.insertVersionSQL(db.tableName), pkgName, version, applied); err != nil {
		return errors.Wrap(err, "failed to insert new migration record")
	}

	return nil
}

// FindMigration finds one migration by the given version ID
func (db *DB) FindMigration(version int64) (*MigrationRecord, error) {
	var row MigrationRecord

	var q = db.dialect.migrationSQL(db.tableName)
	var err = db.QueryRow(q, version).Scan(&row.Time, &row.IsApplied, &row.Time)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		} else {
			return nil, errors.Wrap(err, "failed to query the latest migration")
		}
	}

	return &row, nil
}

func (db *DB) LoadMigrationRecords() ([]MigrationRecord, error) {
	return db.LoadMigrationRecordsByPackage(context.Background(), defaultPackageName)
}

func (db *DB) LoadMigrationRecordsByPackage(ctx context.Context, pkgName string) ([]MigrationRecord, error) {
	rows, err := db.DB.QueryContext(ctx, db.dialect.queryVersionsSQL(db.tableName), pkgName)
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
		if err = rows.Scan(&row.Package, &row.VersionID, &row.IsApplied, &row.Time); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}

		records = append(records, row)
	}

	if err := rows.Err(); err != nil {
		return records, errors.Wrap(err, "failed to read the next row")
	}

	return records, nil
}

// runCoreMigration executes the core migration
func (db *DB) runCoreMigration(ctx context.Context) error {
	tableNames, err := db.getTableNames(ctx)
	if err != nil {
		return err
	}

	log.Infof("found tableNames: %+v", tableNames)

	// check if it's the latest version
	if sliceContains(tableNames, TableName) {
		// if so, we are good

		// check the latest core version
		latestVersion, err := db.queryLatestVersion(ctx, corePackageName)
		if err != nil {
			return err
		}

		log.Infof("found latest core package version: %d", latestVersion)

		return db.upgradeCoreMigrations(ctx, latestVersion)
	} else if sliceContains(tableNames, legacyGooseTableName) {
		// the legacy version
		return db.migrateLegacyGooseTable(ctx)
	}

	// no version table found, create the version table with the latest schema
	return db.createVersionTable(ctx, db, VersionRockhopperV1)
}

func (db *DB) upgradeCoreMigrations(ctx context.Context, currentVersion int64) error {
	if currentVersion < 1 { /* do something */
	}
	if currentVersion < 2 { /* do something */
	}
	return nil
}

// queryLatestVersion selects the latest db version of a package
func (db *DB) queryLatestVersion(ctx context.Context, pkgName string) (int64, error) {
	row := db.DB.QueryRowContext(ctx,
		db.dialect.selectLastVersionSQL(TableName), pkgName)

	if err := row.Err(); err != nil {
		return 0, err
	}

	var versionId sql.NullInt64
	if err := row.Scan(&versionId); err != nil {
		return 0, err
	}

	if versionId.Valid {
		return versionId.Int64, nil
	}

	return 0, nil
}

// migrateLegacyGooseTable migrates the legacy goose version table to the new rockhopper version table
func (db *DB) migrateLegacyGooseTable(ctx context.Context) error {
	if err := db.createVersionTable(ctx, db, VersionRockhopperV1); err != nil {
		return err
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}

	switch db.dialect.(type) {
	case *MySQLDialect, *TiDBDialect:
		if err := execAndCheckErr(tx, ctx,
			`ALTER TABLE goose_db_version ADD COLUMN package VARCHAR(125) NOT NULL DEFAULT 'main'`); err != nil {
			return rollbackAndLogErr(err, tx, "unable to alter table")
		}

		if err := execAndCheckErr(tx, ctx,
			fmt.Sprintf(`RENAME TABLE %s TO %s`, legacyGooseTableName, TableName)); err != nil {
			return rollbackAndLogErr(err, tx, "unable to rename table")
		}

	case *PostgresDialect, *RedshiftDialect:
		if err := execAndCheckErr(tx, ctx,
			`ALTER TABLE goose_db_version ADD COLUMN package VARCHAR(125) NOT NULL DEFAULT 'main'`); err != nil {
			return rollbackAndLogErr(err, tx, "unable to alter table")
		}

		if err := execAndCheckErr(tx, ctx,
			fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, legacyGooseTableName, TableName)); err != nil {
			return rollbackAndLogErr(err, tx, "unable to rename table")
		}

	case *Sqlite3Dialect, *SqlServerDialect:
		if err := execAndCheckErr(tx, ctx,
			fmt.Sprintf(`INSERT INTO %s(id, package, version_id, is_applied, tstamp) SELECT id, 'main', version_id, is_applied, tstamp FROM %s`,
				TableName,
				legacyGooseTableName),
		); err != nil {
			return rollbackAndLogErr(err, tx, "unable to execute insert from select")
		}

		if err := execAndCheckErr(tx, ctx, fmt.Sprintf(`DROP TABLE %s`, legacyGooseTableName)); err != nil {
			return rollbackAndLogErr(err, tx, "unable to drop legacy table")
		}
	}

	return tx.Commit()
}

// Touch checks if the version table exists, if not, create the version table
func (db *DB) Touch(ctx context.Context) error {
	if err := db.runCoreMigration(ctx); err != nil {
		return err
	}

	_, err := db.queryLatestVersion(ctx, corePackageName)
	if err == nil {
		return nil
	}

	// table exists, but there are no rows
	// this is unexpected; the initial core version should exist
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	return db.createVersionTable(ctx, db, VersionRockhopperV1)
}

// CurrentVersion get the current version of the migration version table
func (db *DB) CurrentVersion(ctx context.Context, packageName string) (int64, error) {
	if err := db.Touch(ctx); err != nil {
		return 0, err
	}

	version, err := db.queryLatestVersion(ctx, packageName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}

		return 0, err
	}

	return version, nil
}

// createVersionTable creates the db version table and inserts the initial value 0 into the migration table
func (db *DB) createVersionTable(ctx context.Context, tx SqlExecutor, initVersion int64) error {
	if _, err := tx.ExecContext(ctx, db.dialect.createVersionTableSQL(db.tableName)); err != nil {
		return err
	}

	return db.insertVersion(ctx, tx, corePackageName, initVersion, true)
}

func sliceContains(a []string, b string) bool {
	for _, s := range a {
		if s == b {
			return true
		}
	}
	return false
}

func rollbackAndLogErr(originErr error, txn *sql.Tx, msg string, args ...any) error {
	if err := txn.Rollback(); err != nil {
		log.WithError(err).Errorf("unable to rollback transaction")
		return errors.Wrapf(originErr, msg, args...)
	}

	return originErr
}

func execAndCheckErr(db SqlExecutor, ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.ExecContext(ctx, sql, args...)
	if err != nil {
		log.WithError(err).Errorf("unable to execute SQL: %s", sql)
		return err
	}

	return nil
}
