package rockhopper

import (
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

// SQLDialect abstracts the details of specific SQL dialects. The concrete
// implementations and the CRUD query builder live in pkg/dialect.
type SQLDialect = dialect.Dialect

func LoadDialect(d string) (SQLDialect, error) {
	switch d {
	case DialectPostgres:
		return dialect.NewPostgresDialect(), nil
	case DialectMySQL:
		return dialect.NewMySQLDialect(), nil
	case DialectSQLite3:
		return dialect.NewSqlite3Dialect(), nil
	case DialectRedshift:
		return dialect.NewRedshiftDialect(), nil
	case DialectTiDB:
		return dialect.NewTiDBDialect(), nil
	}

	return nil, fmt.Errorf("%q: unknown dialect", d)
}
