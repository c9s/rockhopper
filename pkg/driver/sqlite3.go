//go:build !no_sqlite3

package driver

import (
	_ "github.com/mattn/go-sqlite3" // register the SQLite database/sql driver
)
