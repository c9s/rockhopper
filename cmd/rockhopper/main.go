package main

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/x-cray/logrus-prefixed-formatter"

	"github.com/c9s/rockhopper"
)

var config *rockhopper.Config

var rootCmd = &cobra.Command{
	Use:   "rh",
	Short: "rockhopper migration tool",
	Long:  "rockhopper is a migration tool written in Go",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configFile := viper.GetString("config")
		_, err := os.Stat(configFile)
		if err != nil && os.IsNotExist(err) {
			return fmt.Errorf("config file %s does not exist", configFile)
		}

		// load config into the global instance
		config, err = rockhopper.LoadConfig(configFile)
		if err != nil {
			return err
			log.Fatal(err)
		}

		return nil
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().Bool("debug", false, "debug flag")
	rootCmd.PersistentFlags().String("config", "rockhopper.yaml", "rockhopper config file")

	// Once the flags are defined, we can bind config keys with flags.
	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Errorf("failed to bind persistent flags. please check the persistent flags settings.")
	}

	viper.SetEnvPrefix("ROCKHOPPER_")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Enable environment variable binding, the env vars are not overloaded yet.
	viper.AutomaticEnv()

	log.SetFormatter(&prefixed.TextFormatter{})
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}
