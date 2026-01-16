package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var removePhaseCmd = &cobra.Command{
	Use:   "remove-phase [phase-number]",
	Short: "Remove a phase from the roadmap",
	Long: `Remove a phase from your project roadmap.

Requires: ROADMAP.md (run 'ralph roadmap' first)

Example:
  ralph remove-phase 5

This removes Phase 5 and renumbers subsequent phases:
  - Phase 6 becomes Phase 5
  - Phase 7 becomes Phase 6
  - etc.

Use with caution - this deletes the phase directory and all plans within.

Accepts both integer (5) and decimal (5.1) phase numbers.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, err := ValidatePhaseNumber(args[0])
		if err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		return gsd.RemovePhase(context.Background(), phase)
	},
}

func init() {
	rootCmd.AddCommand(removePhaseCmd)
}
