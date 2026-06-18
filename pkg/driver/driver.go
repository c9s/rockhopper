// Package driver bundles the supported SQL drivers (MySQL, PostgreSQL and
// SQLite3) so that importing it registers them with the database/sql package.
// Each driver lives in its own file guarded by a build tag (no_mysql,
// no_postgres, no_sqlite3) so it can be excluded at build time.
//
// This file carries no build constraints, ensuring the package always has at
// least one Go file and that NormalizeMySQLDSN is always declared, even when
// every driver is excluded from the build.
package driver

// NormalizeMySQLDSN, when set, rewrites a MySQL DSN so that parseTime=true is
// enabled. rockhopper scans the version table's tstamp column into time.Time,
// which requires parseTime=true; without it the driver returns the raw []byte
// and scanning fails. It is registered by mysql.go's init and stays nil when
// the MySQL driver is excluded from the build (the no_mysql build tag).
var NormalizeMySQLDSN func(dsn string) (string, error)
