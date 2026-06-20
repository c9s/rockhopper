package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/rockhopper/v2/pkg/dialect"
	"github.com/c9s/rockhopper/v2/pkg/driver"
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
			return nil, fmt.Errorf("failed to build dsn from env vars: %w", err)
		}
	}

	return Open(config.Driver, dialect, dsn, TableName)
}

func BuildDSNFromEnvVars(driver string) (string, error) {
	switch driver {
	case DialectMySQL:
		return buildMySqlDSN()

	}
	return "", fmt.Errorf("can not build dsn for driver %s", driver)
}

func castDriverName(driver string) string {
	switch driver {
	case DialectRedshift:
		return DialectPostgres
	case DialectTiDB:
		return DialectMySQL
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
	case DialectPostgres, DialectSQLite3, DialectMySQL:
	default:
		return nil, fmt.Errorf("unsupported driver %s", driverName)
	}

	// Ensure parseTime=true is set for MySQL/TiDB so DATETIME/TIMESTAMP columns
	// scan into time.Time. castDriverName has already folded TiDB into MySQL.
	if driverName == DialectMySQL && dsn != "" && driver.NormalizeMySQLDSN != nil {
		normalized, err := driver.NormalizeMySQLDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mysql dsn: %q: %w", maskDsnPassword(dsn), err)
		}

		dsn = normalized
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection with dsn: %q: %w", maskDsnPassword(dsn), err)
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
	q, args := db.dialect.Delete(db.tableName, []dialect.Col{
		{Name: "package", Val: pkgName},
		{Name: "version_id", Val: version},
	})
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return errors.Wrap(err, "failed to delete migration record")
	}

	return nil
}

func (db *DB) getTableNames(ctx context.Context) ([]string, error) {
	q := db.dialect.TableNames()
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

func (db *DB) insertVersion(ctx context.Context, tx SQLExecutor, pkgName, sourceFile string, version int64, applied bool) error {
	q, args := db.dialect.Insert(db.tableName, []dialect.Col{
		{Name: "package", Val: pkgName},
		{Name: "source_file", Val: sourceFile},
		{Name: "version_id", Val: version},
		{Name: "is_applied", Val: applied},
	})
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return errors.Wrap(err, "failed to insert new migration record")
	}

	return nil
}

// LoadMigration finds the migration record from the db, and then updates the record to
// the given Migration object. The migration.Record field will be updated.
// When returning (nil, nil), it means the record is not found.
func (db *DB) LoadMigration(ctx context.Context, m *Migration) (*Migration, error) {
	var record MigrationRecord

	q, args := db.dialect.Select(db.tableName,
		[]string{"id", "tstamp", "is_applied"},
		[]dialect.Col{
			{Name: "package", Val: m.Package},
			{Name: "version_id", Val: m.Version},
		},
		dialect.SelectOpt{OrderBy: []dialect.Order{{Col: "tstamp", Desc: true}}, Limit: 1})

	row := db.QueryRowContext(ctx, q, args...)
	if err := row.Err(); err != nil {
		return nil, convertNoRowsErrToNil(err)
	}

	var id int64
	var err = row.Scan(&id, &record.Time, &record.IsApplied)
	if err != nil {
		return nil, convertNoRowsErrToNil(err)
	}

	m.Record = &record
	return m, nil
}

// LoadMigrationRecords
//
// Deprecated: use LoadMigrationRecordsByPackage instead
func (db *DB) LoadMigrationRecords() ([]MigrationRecord, error) {
	return db.LoadMigrationRecordsByPackage(context.Background(), DefaultPackageName)
}

func (db *DB) LoadMigrationRecordsByPackage(ctx context.Context, pkgName string) ([]MigrationRecord, error) {
	q, args := db.dialect.Select(db.tableName,
		[]string{"package", "version_id", "is_applied", "tstamp"},
		[]dialect.Col{{Name: "package", Val: pkgName}},
		dialect.SelectOpt{OrderBy: []dialect.Order{{Col: "id", Desc: true}}})

	rows, err := db.QueryContext(ctx, q, args...)
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

	// check if it's the latest version
	if sliceContains(tableNames, TableName) {
		// if so, we are good

		// check the latest core version
		latestVersion, err := db.queryLatestVersion(ctx, CorePackageName)
		if err != nil {
			return err
		}

		log.Debugf("found latest core package version: %d", latestVersion)

		return db.upgradeCoreMigrations(ctx, latestVersion)
	} else if sliceContains(tableNames, legacyGooseTableName) {

		// the legacy version
		log.Debugf("found legacy goose table, migrating...")

		return db.migrateLegacyGooseTable(ctx)
	}

	// no version table found, create the version table with the latest schema
	return db.createVersionTable(ctx, db, VersionRockhopperV1)
}

func (db *DB) upgradeCoreMigrations(_ context.Context, _ int64) error {
	// placeholder for future core migration upgrades
	return nil
}

// queryLatestVersion selects the latest db version of a package
func (db *DB) queryLatestVersion(ctx context.Context, pkgName string) (int64, error) {
	q, args := db.dialect.Select(TableName,
		[]string{"MAX(version_id)"},
		[]dialect.Col{{Name: "package", Val: pkgName}},
		dialect.SelectOpt{})

	row := db.QueryRowContext(ctx, q, args...)

	if err := row.Err(); err != nil {
		return 0, convertNoRowsErrToNil(err)
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

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Add the package column to the legacy table so its rows can be migrated.
	// SQLite reports AddColumn as unsupported and is skipped here (as before).
	if alterSQL, supported := db.dialect.AddColumn(legacyGooseTableName, dialect.Column{
		Name:    "package",
		Type:    dialect.ColVarchar,
		Size:    packageColumnSize,
		NotNull: true,
		Default: "'main'",
	}); supported {
		if err := execAndCheckErr(tx, ctx, alterSQL); err != nil {
			return rollbackAndLogErr(err, tx, "unable to alter table")
		}
	}

	if err := execAndCheckErr(tx, ctx,
		fmt.Sprintf(`INSERT INTO %s(package, version_id, is_applied, tstamp) SELECT 'main', version_id, is_applied, tstamp FROM %s`,
			TableName,
			legacyGooseTableName),
	); err != nil {
		return rollbackAndLogErr(err, tx, "unable to execute insert from select")
	}

	if err := execAndCheckErr(tx, ctx, fmt.Sprintf(`DROP TABLE %s`, legacyGooseTableName)); err != nil {
		return rollbackAndLogErr(err, tx, "unable to drop legacy table")
	}

	return tx.Commit()
}

// Touch checks if the version table exists, if not, create the version table
func (db *DB) Touch(ctx context.Context) error {
	if err := db.runCoreMigration(ctx); err != nil {
		return err
	}

	_, err := db.queryLatestVersion(ctx, CorePackageName)
	return convertNoRowsErrToNil(err)
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
	if _, err := tx.ExecContext(ctx, db.dialect.CreateTable(versionSchema(db.tableName))); err != nil {
		return err
	}

	return db.insertVersion(ctx, tx, CorePackageName, "", initVersion, true)
}

// packageColumnSize is the VARCHAR width of the package identifier column. It is
// shared by the version table, the data-migration table, and the legacy-table
// upgrade ALTER so the three definitions can never drift apart.
const packageColumnSize = 128

// versionSchema describes the migration version table.
func versionSchema(tableName string) dialect.Schema {
	return dialect.Schema{
		Table: tableName,
		Columns: []dialect.Column{
			{Name: "id", Type: dialect.ColSerial, PrimaryKey: true},
			{Name: "package", Type: dialect.ColVarchar, Size: packageColumnSize, NotNull: true, Default: "'main'"},
			{Name: "source_file", Type: dialect.ColVarchar, Size: 255, NotNull: true, Default: "''"},
			{Name: "version_id", Type: dialect.ColBigInt, NotNull: true},
			{Name: "is_applied", Type: dialect.ColBool, NotNull: true},
			{Name: "tstamp", Type: dialect.ColTimestamp, NotNull: true, Default: dialect.DefaultNow},
		},
	}
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
	log.WithError(originErr).Errorf("unable to execute SQL, rolling back...")

	if err := txn.Rollback(); err != nil {
		log.WithError(err).Errorf("unable to rollback transaction")
		if len(msg) > 0 {
			return errors.Wrapf(originErr, msg, args...)
		}

		return errors.Wrap(originErr, err.Error())
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

func convertNoRowsErrToNil(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	return err
}

func (db *DB) FindLastAppliedMigration(
	ctx context.Context, allMigrations MigrationSlice,
) (int, *Migration, error) {
	for i := len(allMigrations) - 1; i >= 0; i-- {
		m := allMigrations[i]

		m, err := db.LoadMigration(ctx, m)
		if err != nil {
			return -1, nil, err
		}

		if m != nil && m.Record != nil && m.Record.IsApplied {
			return i, m, nil
		}
	}

	return -1, nil, nil
}

// MigrationStatus summarizes the applied state of a set of migrations.
type MigrationStatus struct {
	// Pending holds the migrations that have not been applied yet, in the order
	// they appear in the inspected slice (ascending version when sorted).
	Pending MigrationSlice

	// OutOfOrder holds the subset of Pending whose version is lower than the
	// highest already-applied version. A resume-from-last-applied upgrade walks
	// forward from the last applied migration and would silently skip these.
	OutOfOrder MigrationSlice

	// HighestAppliedVersion is the highest version that has been applied, or 0
	// when nothing has been applied yet.
	HighestAppliedVersion int64
}

// InspectMigrations loads the applied state for every migration in the slice and
// reports which migrations are pending and which of those are out of order. The
// slice is expected to be sorted in ascending version order (as produced by
// SortAndConnect); detection itself does not depend on the ordering.
func (db *DB) InspectMigrations(ctx context.Context, migrations MigrationSlice) (*MigrationStatus, error) {
	status := &MigrationStatus{}

	for _, m := range migrations {
		// reset any stale record before loading so a not-found lookup is treated as pending
		m.Record = nil

		if _, err := db.LoadMigration(ctx, m); err != nil {
			return nil, err
		}

		if m.Record != nil && m.Record.IsApplied && m.Version > status.HighestAppliedVersion {
			status.HighestAppliedVersion = m.Version
		}
	}

	for _, m := range migrations {
		if m.Record != nil && m.Record.IsApplied {
			continue
		}

		status.Pending = append(status.Pending, m)

		if m.Version < status.HighestAppliedVersion {
			status.OutOfOrder = append(status.OutOfOrder, m)
		}
	}

	return status, nil
}

// maskDsnPassword mask the password from the DSN string
func maskDsnPassword(dsn string) string {
	d, err := mysql.ParseDSN(dsn) // just to validate the DSN, ignore the error
	if err != nil {
		return dsn
	}

	d.Passwd = "******"
	return d.FormatDSN()
}

func getPassword() string {
	return os.Getenv("ROCKHOPPER_PASSWORD")
}
