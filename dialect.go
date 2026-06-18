package rockhopper

import (
	"database/sql"
	"fmt"

	"github.com/c9s/rockhopper/v2/pkg/dialect"
)

const (
	DialectPostgres = "postgres"
	DialectMySQL    = "mysql"
	DialectSQLite3  = "sqlite3"
	DialectRedshift = "redshift"
	DialectTiDB     = "tidb"
)

// SQLDialect abstracts the details of specific SQL dialects
// for goose's few SQL specific statements
type SQLDialect interface {
	GetTableNamesSQL() string                      // return the sql string to get the table names
	CreateVersionTableSQL(tableName string) string // sql string to create the db version table
	InsertVersionSQL(tableName string) string      // sql string to insert the initial version table row
	DeleteVersionSQL(tableName string) string      // sql string to delete version

	// MigrationSQL returns the sql string to retrieve migrations
	MigrationSQL(tableName string) string

	// SelectLastVersionSQL returns the sql string to get the latest version
	SelectLastVersionSQL(tableName string) string

	// QueryVersionsSQL returns the sql string to query the version info descending
	QueryVersionsSQL(tableName string) string
	DBVersionQuery(db *sql.DB, tableName string) (*sql.Rows, error)

	// CreateDataMigrationTableSQL returns the sql string to create the
	// data-migration state table (checkpoint/status tracking).
	CreateDataMigrationTableSQL(tableName string) string

	// InsertDataMigrationSQL returns the sql string to insert a new
	// data-migration state row. Argument order: package, version_id, name,
	// status, checkpoint.
	InsertDataMigrationSQL(tableName string) string

	// SelectDataMigrationSQL returns the sql string to load the status and
	// checkpoint of a data migration. Argument order: package, version_id.
	SelectDataMigrationSQL(tableName string) string

	// AcquireDataMigrationLeaseSQL returns the sql string to conditionally
	// claim the lease (when unowned, self-owned or expired). Argument order:
	// owner, expires_at, package, version_id, owner, now. A claim succeeded
	// when exactly one row was affected.
	AcquireDataMigrationLeaseSQL(tableName string) string

	// CommitDataBatchSQL returns the sql string to persist a batch's status and
	// checkpoint while renewing the lease, guarded by ownership. Argument
	// order: status, checkpoint, expires_at, package, version_id, owner. The
	// caller still holds the lease when exactly one row was affected.
	CommitDataBatchSQL(tableName string) string

	// ReleaseDataMigrationLeaseSQL returns the sql string to set a terminal
	// status and clear the lease, guarded by ownership. Argument order: status,
	// package, version_id, owner.
	ReleaseDataMigrationLeaseSQL(tableName string) string
}

func LoadDialect(d string) (SQLDialect, error) {
	switch d {
	case DialectPostgres:
		return &dialect.PostgresDialect{}, nil
	case DialectMySQL:
		return &dialect.MySQLDialect{}, nil
	case DialectSQLite3:
		return &dialect.Sqlite3Dialect{}, nil
	case DialectRedshift:
		return &dialect.RedshiftDialect{}, nil
	case DialectTiDB:
		return &dialect.TiDBDialect{}, nil
	}

	return nil, fmt.Errorf("%q: unknown dialect", d)
}
