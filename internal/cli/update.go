package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Conversational roadmap updates based on findings",
	Long: `Have a conversation about changes to your roadmap.

Requires: ROADMAP.md (run 'ralph roadmap' first)

Tell Claude about:
  - New findings or discoveries
  - Scope changes
  - Work that needs to be added or removed

Claude will decide the best way to update your roadmap:
  - Add to existing phases
  - Create new phases
  - Insert urgent phases
  - Remove obsolete phases

This is the recommended way to modify your roadmap. It replaces
manual use of add-phase, insert-phase, and remove-phase commands
with an intelligent agent that figures out the right approach.

Your input is captured verbatim for context during planning.

Note: Currently updates ROADMAP.md (markdown). Phase 5 will migrate to roadmap.json.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		// TODO(Phase 5): After GSD integration, this should validate/convert ROADMAP.md to roadmap.json
		gsd := planner.NewGSD("", cwd)
		return gsd.UpdateRoadmap(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
