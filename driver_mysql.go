//go:build !no_mysql
// +build !no_mysql

package rockhopper

import (
	_ "github.com/go-sql-driver/mysql"
)
