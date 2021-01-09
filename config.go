package rockhopper

import (
	"io/ioutil"

	"github.com/codingconcepts/env"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Driver        string `json:"driver" yaml:"driver" env:"ROCKHOPPER_DRIVER"`
	Dialect       string `json:"dialect" yaml:"dialect" env:"ROCKHOPPER_DIALECT"`
	DSN           string `json:"dsn" yaml:"dsn" env:"ROCKHOPPER_DSN"`
	MigrationsDir string `json:"migrationsDir" env:"ROCKHOPPER_MIGRATIONS_DIR"`
}

func LoadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
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

	return &config, err
}
