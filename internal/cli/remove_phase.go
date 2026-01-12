package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

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

Use with caution - this deletes the phase directory and all plans within.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid phase number: %s", args[0])
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
