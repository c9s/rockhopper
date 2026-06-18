//go:build !no_postgres
// +build !no_postgres

package driver

import (
	_ "github.com/lib/pq"
)
