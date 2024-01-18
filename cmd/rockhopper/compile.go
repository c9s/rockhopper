package main

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper"
)

func init() {
	CompileCmd.Flags().StringArrayP("package", "p", []string{}, "package")
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

	skipBuild, err := cmd.Flags().GetBool("no-build")
	if err != nil {
		return err
	}

	includePackages, err := cmd.Flags().GetStringArray("package")
	if err != nil {
		return err
	}

	if !dirExists(outputDir) {
		if err := os.MkdirAll(outputDir, 0777); err != nil {
			return err
		}
	}

	loader := rockhopper.NewSqlMigrationLoader(config)

	allMigrations, err := loader.Load(config.MigrationsDirs...)
	if err != nil {
		return err
	}

	if len(allMigrations) == 0 {
		log.Infof("no migrations found")
		return nil
	}

	if len(includePackages) > 0 {
		log.Infof("include packages: %v", includePackages)
		allMigrations = allMigrations.FilterPackage(includePackages)
	}

	var dumper = &rockhopper.GoMigrationDumper{
		Dir:  outputDir,
		Wipe: true,
	}

	if err := dumper.Dump(allMigrations); err != nil {
		return err
	}

	if skipBuild {
		return nil
	}

	// test compile
	buildCmd := exec.Command("go", "build", "./"+outputDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	return buildCmd.Run()
}
