package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Analyze existing codebase structure",
	Long: `Analyze an existing codebase to understand its structure.

Best used BEFORE 'ralph init' for brownfield projects.

This launches parallel Explore agents to analyze your code and creates:
  .planning/codebase/
  ├── STACK.md          Technology stack
  ├── ARCHITECTURE.md   Patterns and layers
  ├── STRUCTURE.md      Directory layout
  ├── CONVENTIONS.md    Code style
  ├── TESTING.md        Test setup
  ├── INTEGRATIONS.md   External APIs
  └── CONCERNS.md       Tech debt and issues`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		return gsd.MapCodebase(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(mapCmd)
}
