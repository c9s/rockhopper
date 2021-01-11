package main

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	DownCmd.Flags().String("to", "", "downgrade to a specific version")
	DownCmd.Flags().Int("steps", 0, "downgrade by steps")
	RootCmd.AddCommand(DownCmd)
}

var DownCmd = &cobra.Command{
	Use:   "down",
	Short: "downgrade database",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         down,
}

func down(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	to, err := cmd.Flags().GetInt64("to")
	if err != nil {
		return err
	}

	db, err := rockhopper.OpenByConfig(config)
	if err != nil {
		return err
	}

	defer db.Close()

	loader := &rockhopper.SqlMigrationLoader{}
	migrations, err := loader.Load(config.MigrationsDir)
	if err != nil {
		return err
	}

	currentVersion, err := db.CurrentVersion()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return rockhopper.Down(ctx, db, migrations, currentVersion, to, func(m *rockhopper.Migration) {
		log.Infof("migration %v is rolled back", m.Version)
	})
}


