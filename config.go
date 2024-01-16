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

	MigrationsDir string `json:"migrationsDir" yaml:"migrationsDir" env:"ROCKHOPPER_MIGRATIONS_DIR"`

	MigrationsDirs []string `json:"migrationsDirs" yaml:"migrationsDirs" env:"ROCKHOPPER_MIGRATIONS_DIRS"`

	TableName string `json:"tableName" yaml:"tableName" env:"ROCKHOPPER_TABLE_NAME"`

	// Packages is the migration packages, optional
	Packages []string `json:"packages" yaml:"packages"`
}

func LoadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

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
