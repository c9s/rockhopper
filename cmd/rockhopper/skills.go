package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	SkillsInstallCmd.Flags().StringP("dir", "d", ".", "target project directory")
	SkillsInstallCmd.Flags().BoolP("force", "f", false, "overwrite existing skill files")

	SkillsCmd.AddCommand(SkillsInstallCmd)
	SkillsCmd.AddCommand(SkillsListCmd)
	rootCmd.AddCommand(SkillsCmd)
}

var SkillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "manage the bundled Claude Code skills",
	Long: "manage the Claude Code skills bundled with rockhopper.\n\n" +
		"Use `rockhopper skills install` from a project that uses rockhopper to scaffold\n" +
		"AI-assisted migration skills into its .claude/skills directory. The skills are\n" +
		"versioned with the binary, so re-running install after a rockhopper upgrade keeps\n" +
		"them in sync with the CLI.",

	// skills commands must work without a config file
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var SkillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "list the bundled Claude Code skills",

	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := rockhopper.SkillNames()
		if err != nil {
			return err
		}

		for _, name := range names {
			fmt.Println(name)
		}

		return nil
	},
}

var SkillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "install the bundled Claude Code skills into a project's .claude/skills directory",

	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := cmd.Flags().GetString("dir")
		if err != nil {
			return err
		}

		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}

		written, skipped, err := rockhopper.InstallSkills(dir, force)
		if err != nil {
			return err
		}

		for _, p := range written {
			fmt.Printf("installed %s\n", p)
		}

		for _, p := range skipped {
			fmt.Printf("skipped   %s (already exists)\n", p)
		}

		fmt.Printf("\n%d skill file(s) installed, %d skipped\n", len(written), len(skipped))
		if len(skipped) > 0 {
			fmt.Println("re-run with --force to overwrite existing files")
		}

		return nil
	},
}
