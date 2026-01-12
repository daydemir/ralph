package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var roadmapCmd = &cobra.Command{
	Use:   "roadmap",
	Short: "Create phase breakdown for your project",
	Long: `Create a roadmap that breaks your project into phases.

Requires: PROJECT.md (run 'ralph init' first)

This opens Claude to analyze your project and creates:
  .planning/
  ├── ROADMAP.md     Phase breakdown with goals and dependencies
  ├── STATE.md       Current position tracking
  └── phases/        Directory structure for each phase

After roadmap, run 'ralph plan 1' to create plans for the first phase.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		return gsd.CreateRoadmap(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(roadmapCmd)
}
