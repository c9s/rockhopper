package main

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/x-cray/logrus-prefixed-formatter"

	"github.com/c9s/rockhopper"
)

var RootCmd = &cobra.Command{
	Use:   "rh",
	Short: "rockhopper migration tool",
	Long:  "rockhopper is a migration tool written in Go",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().Bool("debug", false, "debug flag")
	RootCmd.PersistentFlags().String("config", "rockhopper.yaml", "config file")
}

func main() {
	viper.SetEnvPrefix("ROCKHOPPER_")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Enable environment variable binding, the env vars are not overloaded yet.
	viper.AutomaticEnv()

	// Once the flags are defined, we can bind config keys with flags.
	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Errorf("failed to bind persistent flags. please check the flag settings.")
	}

	if err := viper.BindPFlags(RootCmd.Flags()); err != nil {
		log.WithError(err).Errorf("failed to bind local flags. please check the flag settings.")
	}

	log.SetFormatter(&prefixed.TextFormatter{})

	configFile := viper.GetString("config")
	_, err := os.Stat(configFile)
	if err != nil && os.IsNotExist(err) {
		log.Fatalf("config file %s does not exist", configFile)
	}

	config, err := rockhopper.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	_ = config

	if err := RootCmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}
