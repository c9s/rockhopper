package rockhopper

import (
	"os"

	"github.com/codingconcepts/env"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Driver  string `json:"driver" yaml:"driver" env:"ROCKHOPPER_DRIVER"`
	Dialect string `json:"dialect" yaml:"dialect" env:"ROCKHOPPER_DIALECT"`
	DSN     string `json:"dsn" yaml:"dsn" env:"ROCKHOPPER_DSN"`

	Package string `json:"package" yaml:"package"`

	MigrationsDir string `json:"migrationsDir" yaml:"migrationsDir" env:"ROCKHOPPER_MIGRATIONS_DIR"`

	MigrationsDirs []string `json:"migrationsDirs" yaml:"migrationsDirs" env:"ROCKHOPPER_MIGRATIONS_DIRS"`

	TableName string `json:"tableName" yaml:"tableName" env:"ROCKHOPPER_TABLE_NAME"`

	// IncludePackages is used as a whitelist for the migration packages, optional
	IncludePackages []string `json:"includePackages" yaml:"includePackages"`
}

func LoadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// Expand environment variables referenced in the config file before parsing,
	// so values can be pulled from the environment, e.g.:
	//
	//   dsn: ${MYSQL8_URL}
	//   dsn: $MYSQL8_URL
	//
	// Both $VAR and ${VAR} forms are supported. Undefined variables expand to an
	// empty string (same as os.ExpandEnv). A literal '$' can be escaped as '$$'.
	data = []byte(expandEnv(string(data)))

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if err := env.Set(&config); err != nil {
		return nil, err
	}

	if len(config.MigrationsDir) > 0 {
		config.MigrationsDirs = append(config.MigrationsDirs, config.MigrationsDir)
	}

	return &config, err
}

// expandEnv replaces $VAR and ${VAR} references in s with the values of the
// corresponding environment variables. It behaves like os.ExpandEnv, with one
// addition: "$$" expands to a literal "$" so values that legitimately contain a
// dollar sign (e.g. a password) can be escaped.
func expandEnv(s string) string {
	return os.Expand(s, func(name string) string {
		if name == "$" {
			return "$"
		}

		return os.Getenv(name)
	})
}
