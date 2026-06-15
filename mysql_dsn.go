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

	if v, ok := os.LookupEnv(prefix + "DATABASE"); ok {
		dsn += "/" + v
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
