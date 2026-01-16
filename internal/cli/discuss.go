package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var discussCmd = &cobra.Command{
	Use:   "discuss [phase-number]",
	Short: "Discuss a phase before planning",
	Long: `Have an interactive discussion about a phase before creating plans.

Requires: ROADMAP.md (run 'ralph roadmap' first)

This opens a conversation to explore scope, approaches, and concerns.
Creates: .planning/phases/{phase}/CONTEXT.md

Great for getting alignment on implementation approach before committing.
After discussion, run 'ralph plan N' to create executable plans.

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
		return gsd.DiscussPhase(context.Background(), phase)
	},
}

func init() {
	rootCmd.AddCommand(discussCmd)
}
