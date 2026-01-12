package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new project with GSD planning",
	Long: `Initialize a new project using Get Shit Done (GSD) planning system.

This opens Claude to ask questions about your project and creates:
  .planning/
  ├── PROJECT.md     Project vision, requirements, constraints
  └── config.json    GSD configuration

After init, run 'ralph roadmap' to create your phase breakdown.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		return gsd.NewProject(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
