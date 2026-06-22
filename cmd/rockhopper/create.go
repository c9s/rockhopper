package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	CreateCmd.Flags().StringP("type", "t", "sql", "migration type, could be \"go\" or \"sql\"")
	CreateCmd.Flags().StringP("output", "o", "", "output directory")
	rootCmd.AddCommand(CreateCmd)
}

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create",
	Long:  "create",
	Args:  cobra.MinimumNArgs(1),

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         create,
}

func create(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	templateType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}

	outputDir, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	if outputDir == "" {
		outputDir = defaultMigrationsDir(config)
	}

	if !dirExists(outputDir) {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
	}

	return rockhopper.CreateWithTemplate(outputDir, nil, args[0], templateType)
}

// defaultMigrationsDir resolves the directory new migration files are written to
// when --output is not given. LoadConfig migrates the legacy goose-compatible
// `migrationsDir` into `migrationsDirs`, so that list is the single source of
// truth here; the first directory is used. It falls back to "migrations" when no
// directory is configured.
func defaultMigrationsDir(config *rockhopper.Config) string {
	if len(config.MigrationsDirs) > 0 {
		return config.MigrationsDirs[0]
	}

	return "migrations"
}

func dirExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return info.IsDir()
}
