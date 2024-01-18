package rockhopper

import "os"

// buildMySqlDSN builds the data source name from environment variables
func buildMySqlDSN() (string, error) {
	if v, ok := os.LookupEnv("MYSQL_URL"); ok {
		return v, nil
	}

	if v, ok := os.LookupEnv("MYSQL_DSN"); ok {
		return v, nil
	}

	dsn := ""
	user := "root"

	if v, ok := os.LookupEnv("MYSQL_USER"); ok {
		user = v
		dsn += v
	}

	if v, ok := os.LookupEnv("MYSQL_PASSWORD"); ok {
		dsn += ":" + v
	} else if v, ok := os.LookupEnv("MYSQL_PASS"); ok {
		dsn += ":" + v
	} else if user == "root" {
		if v, ok := os.LookupEnv("MYSQL_ROOT_PASSWORD"); ok {
			dsn = ":" + v
		}
	}

	address := ""
	if v, ok := os.LookupEnv("MYSQL_HOST"); ok {
		address = v
	}

	if v, ok := os.LookupEnv("MYSQL_PORT"); ok {
		address += ":" + v
	}

	if v, ok := os.LookupEnv("MYSQL_PROTOCOL"); ok {
		dsn += v + "(" + address + ")"
	} else {
		dsn += "tcp(" + address + ")"
	}

	if v, ok := os.LookupEnv("MYSQL_DATABASE"); ok {
		dsn += "/" + v
	}

	return dsn, nil
}
