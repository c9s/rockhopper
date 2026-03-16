//go:build !no_postgres
// +build !no_postgres

package rockhopper

import (
	_ "github.com/lib/pq"
)
