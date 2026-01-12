package cli

import (
	"github.com/daydemir/ralph/internal/workspace"
	"github.com/spf13/cobra"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Ralph workspace",
	Long: `Initialize a new Ralph workspace in the current directory.

Creates .ralph/ folder with:
  - config.yaml      Configuration settings
  - prd.json         PRD backlog (empty)
  - prompts/         Customizable prompt templates
  - codebase-map.md  Project structure documentation
  - progress.txt     Agent memory/learnings
  - fix_plan.md      Known issues to address`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return workspace.Init(initForce)
	},
}

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing workspace")
	rootCmd.AddCommand(initCmd)
}
