package rockhopper

import (
	"context"
	"database/sql"
)

// SqlExecutor executes SQL statements with and without a context. Its legacy
// spelling is retained because SQLExecutor names a distinct public interface.
//
//revive:disable-next-line var-naming // Keep the existing exported name for compatibility; SQLExecutor has a different contract.
type SqlExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
