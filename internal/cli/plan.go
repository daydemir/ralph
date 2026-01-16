package cli

import (
	"context"
	"os"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan [phase-number]",
	Short: "Create executable plans for a phase",
	Long: `Create executable PLAN.md files for a phase.

Requires: ROADMAP.md (run 'ralph roadmap' first)

This opens Claude to create plans with:
  - 2-3 atomic tasks per plan (sized for one Claude session)
  - Verification commands for each task
  - Overall verification checks

Creates:
  .planning/phases/{phase}/
  ├── {phase}-01-PLAN.md    First plan
  ├── {phase}-02-PLAN.md    Second plan (if needed)
  └── ...

After planning, run 'ralph run' to execute the plans.

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
		return gsd.PlanPhase(context.Background(), phase)
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
}
