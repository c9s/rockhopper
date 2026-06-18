//go:build !no_mysql
// +build !no_mysql

package driver

import (
	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

// Importing the driver (even via this non-blank import) runs its init and
// registers the "mysql" driver with database/sql. We additionally register a
// DSN normalizer so Open can guarantee parseTime=true.
func init() {
	NormalizeMySQLDSN = func(dsn string) (string, error) {
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			return dsn, err
		}

		if !cfg.ParseTime {
			cfg.ParseTime = true
			log.Debug("mysql dsn: enabling parseTime=true so DATETIME/TIMESTAMP columns scan into time.Time")
		}

		return cfg.FormatDSN(), nil
	}
}
