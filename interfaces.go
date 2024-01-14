package rockhopper

import (
	"context"
	"database/sql"
)

type SqlExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
