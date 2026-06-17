package rockhopper

import (
	"os"
	"strings"
)

// buildMySqlDSN builds the data source name from environment variables.
//
// By default it reads MYSQL_ prefixed environment variables. If any MYSQL8_
// prefixed environment variable is configured, the MYSQL8_ prefix is used
// instead. This lets a dedicated MySQL 8 configuration coexist with the
// default one without overriding it.
func buildMySqlDSN() (string, error) {
	prefix := "MYSQL_"
	if hasEnvWithPrefix("MYSQL8_") {
		prefix = "MYSQL8_"
	}

	if v, ok := os.LookupEnv(prefix + "URL"); ok {
		return v, nil
	}

	if v, ok := os.LookupEnv(prefix + "DSN"); ok {
		return v, nil
	}

	dsn := ""
	user := "root"

	if v, ok := os.LookupEnv(prefix + "USER"); ok {
		user = v
		dsn += v
	}

	if v, ok := os.LookupEnv(prefix + "PASSWORD"); ok {
		dsn += ":" + v
	} else if v, ok := os.LookupEnv(prefix + "PASS"); ok {
		dsn += ":" + v
	} else if user == "root" {
		if v, ok := os.LookupEnv(prefix + "ROOT_PASSWORD"); ok {
			dsn = ":" + v
		}
	}

	// Separate the credentials from the address with '@', per the go-sql-driver
	// DSN format user:pass@protocol(address)/dbname. Only add it when some
	// credentials were provided; otherwise the DSN starts with the protocol.
	if dsn != "" {
		dsn += "@"
	}

	// MYSQL_UNIX_PORT is the standard MySQL environment variable for the Unix
	// socket file path. When set, connect over the socket, e.g.:
	//
	//   root:pass@unix(/opt/local/var/run/mysql8/mysqld.sock)/test
	//
	// It takes precedence over HOST/PORT/PROTOCOL, mirroring how the MySQL
	// client tools prefer the socket when one is configured.
	if socket, ok := os.LookupEnv(prefix + "UNIX_PORT"); ok {
		dsn += "unix(" + socket + ")"
	} else {
		address := ""
		if v, ok := os.LookupEnv(prefix + "HOST"); ok {
			address = v
		}

		if v, ok := os.LookupEnv(prefix + "PORT"); ok {
			address += ":" + v
		}

		if v, ok := os.LookupEnv(prefix + "PROTOCOL"); ok {
			dsn += v + "(" + address + ")"
		} else {
			dsn += "tcp(" + address + ")"
		}
	}

	// The slash separating the database name is always required by the
	// go-sql-driver DSN format, even when no database is selected.
	dsn += "/"
	if v, ok := os.LookupEnv(prefix + "DATABASE"); ok {
		dsn += v
	}

	return dsn, nil
}

// hasEnvWithPrefix reports whether any environment variable name starts with
// the given prefix.
func hasEnvWithPrefix(prefix string) bool {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			return true
		}
	}

	return false
}
