package main

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	CompileCmd.Flags().StringP("output", "o", "pkg/migrations", "path to the migrations package")
	CompileCmd.Flags().BoolP("no-build", "B", false, "do not build the migration package")
	rootCmd.AddCommand(CompileCmd)
}

var CompileCmd = &cobra.Command{
	Use:   "compile",
	Short: "compile sql migration files into a go package",
	Long:  "compile sql migration files into a go package",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         compile,
}

func compile(cmd *cobra.Command, args []string) error {
	if err := checkConfig(config); err != nil {
		return err
	}

	outputDir, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	var loader rockhopper.SqlMigrationLoader
	migrations, err := loader.Load(config.MigrationsDir)
	if err != nil {
		return err
	}

	err = os.Mkdir(outputDir, 0777)
	if err != nil && !os.IsExist(err) {
		return err
	}

	var dumper = rockhopper.GoMigrationDumper{Dir: outputDir}
	if err := dumper.Dump(migrations) ; err != nil {
		return err
	}

	// test compile
	buildCmd := exec.Command("go", "build", "./"+outputDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return err
	}

	return nil
}
