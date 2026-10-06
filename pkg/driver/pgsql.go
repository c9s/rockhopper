//go:build !no_postgres

package driver

import (
	_ "github.com/lib/pq" // register the PostgreSQL database/sql driver
)
