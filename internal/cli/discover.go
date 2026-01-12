package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover [phase-number]",
	Short: "Research a phase before planning",
	Long: `Research implementation approaches for a phase before creating plans.

Requires: ROADMAP.md (run 'ralph roadmap' first)

This researches ecosystem options, finds relevant docs, and creates:
  .planning/phases/{phase}/RESEARCH.md

Recommended for complex or unfamiliar domains (3D, ML, audio, etc.).
After research, run 'ralph plan N' to create executable plans.`,
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
		return gsd.ResearchPhase(context.Background(), phase)
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
