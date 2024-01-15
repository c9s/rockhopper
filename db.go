package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// legacyGooseTableName is the legacy table name
const legacyGooseTableName = "goose_db_version"

// defaultRockhopperTableName is the new version table name
const defaultRockhopperTableName = "rockhopper_versions"

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

	dsn := config.DSN
	if len(dsn) == 0 {
		dsn, err = BuildDSNFromEnvVars(config.Driver)
		if err != nil {
			return nil, errors.Wrap(err, "dsn is not defined, can not build dsn")
		}
	}

	return Open(config.Driver, dialect, dsn, defaultRockhopperTableName)
}

func BuildDSNFromEnvVars(driver string) (string, error) {
	switch driver {
	case "mysql":
		return buildMySqlDSN()

	}
	return "", fmt.Errorf("can not build dsn for driver %s", driver)
}

// buildMySqlDSN builds the data source name from environment variables
func buildMySqlDSN() (string, error) {
	if v, ok := os.LookupEnv("MYSQL_URL"); ok {
		return v, nil
	}

	if v, ok := os.LookupEnv("MYSQL_DSN"); ok {
		return v, nil
	}

	dsn := ""
	user := "root"

	if v, ok := os.LookupEnv("MYSQL_USER"); ok {
		user = v
		dsn += v
	}

	if v, ok := os.LookupEnv("MYSQL_PASSWORD"); ok {
		dsn += ":" + v
	} else if v, ok := os.LookupEnv("MYSQL_PASS"); ok {
		dsn += ":" + v
	} else if user == "root" {
		if v, ok := os.LookupEnv("MYSQL_ROOT_PASSWORD"); ok {
			dsn = ":" + v
		}
	}

	address := ""
	if v, ok := os.LookupEnv("MYSQL_HOST"); ok {
		address = v
	}

	if v, ok := os.LookupEnv("MYSQL_PORT"); ok {
		address += ":" + v
	}

	if v, ok := os.LookupEnv("MYSQL_PROTOCOL"); ok {
		dsn += v + "(" + address + ")"
	} else {
		dsn += "tcp(" + address + ")"
	}

	if v, ok := os.LookupEnv("MYSQL_DATABASE"); ok {
		dsn += "/" + v
	}

	return dsn, nil
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

func New(driverName string, dialect SQLDialect, db *sql.DB, tableName string) *DB {
	return &DB{
		dialect:    dialect,
		driverName: driverName,
		DB:         db,
		tableName:  tableName,
	}
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
	var err = db.QueryRow(q, version).Scan(&row.Time, &row.IsApplied)

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
		if err = rows.Scan(&row.Package, &row.VersionID, &row.IsApplied); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}

		records = append(records, row)
	}

	if err := rows.Err(); err != nil {
		return records, errors.Wrap(err, "failed to read the next row")
	}

	return records, nil
}

func (db *DB) MigrateCore(ctx context.Context) error {
	tableNames, err := db.getTableNames(ctx)
	if err != nil {
		return err
	}

	// check if it's the latest version
	if sliceContains(tableNames, defaultRockhopperTableName) {
		// we are good
		// TODO: check the core version row
		return nil
	} else if sliceContains(tableNames, legacyGooseTableName) {
		// the legacy version
		return db.migrateGooseTable(ctx)
	}

	// no version table found, create the version table with the latest schema
	txn, txnErr := db.Begin()
	if txnErr != nil {
		return txnErr
	}

	if err := db.createVersionTable(ctx, txn, db.tableName, VersionRockhopperV1); err != nil {
		return err
	}

	return txn.Commit()
}

// migrateGooseTable migrates the legacy goose version table to the new rockhopper version table
func (db *DB) migrateGooseTable(ctx context.Context) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}

	if err := db.createVersionTable(ctx, tx, defaultRockhopperTableName, VersionRockhopperV1); err != nil {
		return err
	}

	switch db.dialect.(type) {
	case *MySQLDialect, *TiDBDialect:
		if err := execAndCheckErr(tx, ctx,
			`ALTER TABLE goose_db_version ADD COLUMN package VARCHAR(125) NOT NULL DEFAULT 'main'`); err != nil {
			return rollbackAndLogErr(err, tx, "unable to alter table")
		}

		if err := execAndCheckErr(tx, ctx,
			fmt.Sprintf(`RENAME TABLE %s TO %s`, legacyGooseTableName, defaultRockhopperTableName)); err != nil {
			return rollbackAndLogErr(err, tx, "unable to rename table")
		}

	case *Sqlite3Dialect, *PostgresDialect, *RedshiftDialect, *SqlServerDialect:
		if err := execAndCheckErr(tx, ctx,
			fmt.Sprintf(`INSERT INTO %s(id, package, version_id, is_applied, tstamp) SELECT id, 'main', version_id, is_applied, tstamp FROM %s`,
				defaultRockhopperTableName,
				legacyGooseTableName),
		); err != nil {
			return rollbackAndLogErr(err, tx, "unable to execute insert from select")
		}

		if err := execAndCheckErr(tx, ctx, fmt.Sprintf(`DROP TABLE %s`, legacyGooseTableName)); err != nil {
			return rollbackAndLogErr(err, tx, "unable to drop legacy table")
		}

	default:

	}

	return tx.Commit()
}

// CurrentVersion get the current version of the migration version table
func (db *DB) CurrentVersion() (int64, error) {
	if err := db.MigrateCore(context.Background()); err != nil {
		return 0, err
	}

	rows, err := db.dialect.dbVersionQuery(db.DB, db.tableName)
	if err != nil {
		// table exists, but there is no rows
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}

		txn, txnErr := db.Begin()
		if txnErr != nil {
			return 0, txnErr
		}

		if err := db.createVersionTable(context.Background(), txn, db.tableName, VersionRockhopperV1); err != nil {
			return 0, rollbackAndLogErr(err, txn, "unable to create versions table")
		}

		return VersionRockhopperV1, txn.Commit()
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

const VersionGoose = 0
const VersionRockhopperV1 = 1

// createVersionTable creates the db version table and inserts the initial value 0 into the migration table
func (db *DB) createVersionTable(ctx context.Context, txn *sql.Tx, tableName string, initVersion int) error {
	if _, err := txn.ExecContext(ctx, db.dialect.createVersionTableSQL(tableName)); err != nil {
		return rollbackAndLogErr(err, txn, "unable to create version table")
	}

	if err := db.insertInitialVersion(ctx, txn, tableName, initVersion); err != nil {
		return rollbackAndLogErr(err, txn, "unable to insert version")
	}

	return nil
}

func (db *DB) insertInitialVersion(ctx context.Context, txn SqlExecutor, tableName string, initVersion int) error {
	_, err := txn.ExecContext(ctx, db.dialect.insertVersionSQL(tableName), corePackageName, initVersion, true)
	return err
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
