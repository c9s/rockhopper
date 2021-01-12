package main

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	UpCmd.Flags().Int64("to", 0, "up to a specific version")
	UpCmd.Flags().Int("steps", 0, "run upgrade by steps")
	rootCmd.AddCommand(UpCmd)
}

var UpCmd = &cobra.Command{
	Use:   "up",
	Short: "upgrade database",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         up,
}

func up(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	steps, err := cmd.Flags().GetInt("steps")
	if err != nil {
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

	if steps > 0 {
		return rockhopper.UpBySteps(ctx, db, migrations, currentVersion, steps, func(m *rockhopper.Migration) {
			log.Infof("migration %v is applied", m.Version)
		})
	}

	return rockhopper.Up(ctx, db, migrations, currentVersion, to, func(m *rockhopper.Migration) {
		log.Infof("migration %v is applied", m.Version)
	})

}
