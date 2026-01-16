package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review [phase-number]",
	Short: "Review all plans in a phase before execution",
	Long: `Walk through each plan in a phase to review tasks and verifications.

Requires: Plans exist for the phase (run 'ralph plan N' first)

For each plan, you'll see:
  - The objective
  - Task summaries (what each accomplishes)
  - All verification steps (easy to scan)
  - Option to expand any task for full details

Your feedback is captured verbatim AND used to update plans directly.
This is the quality gate between planning and execution.

After reviewing, run 'ralph run' to start execution.

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
		return gsd.ReviewPlans(context.Background(), phase)
	},
}

func init() {
	rootCmd.AddCommand(reviewCmd)
}
