package rockhopper

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/datatype"
)

type Config struct {
	Driver         string               `json:"driver" yaml:"driver"`
	Dialect        string               `json:"dialect" yaml:"dialect"`
	DSN            string               `json:"dsn" yaml:"dsn"`
	MigrationsDirs datatype.StringSlice `json:"migrationsDirs"`
}

func LoadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return &config, err
}
