package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/daydemir/ralph/internal/display"
	"github.com/daydemir/ralph/internal/executor"
	"github.com/daydemir/ralph/internal/planner"
	"github.com/daydemir/ralph/internal/state"
	"github.com/spf13/cobra"
)

var (
	runLoop         int
	runModel        string
	runSkipAnalysis bool
	maxRetries      int
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the next incomplete plan",
	Long: `Execute plans using Claude Code with automated verification.

Single execution (default):
  ralph run

  Executes the next incomplete plan and stops.

Autonomous loop:
  ralph run --loop 10     (10 iterations)
  ralph run -l 20         (20 iterations)

  Runs multiple plans automatically until:
  - All plans complete
  - A verification fails
  - Max iterations reached

Each plan gets a fresh Claude context for optimal quality.
Verification failures stop the loop immediately.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		// Check prerequisites
		if err := executor.CheckClaudeInstalled(); err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)

		// Check for required planning artifacts
		if !gsd.HasRoadmap() {
			return fmt.Errorf(`no ROADMAP.md found

Ralph requires proper planning before execution.
Run 'ralph roadmap' first to create your phase breakdown.
Then 'ralph plan 1' to create plans for Phase 1.`)
		}

		planningDir := gsd.PlanningDir()

		// Load phases and find next plan
		phases, err := state.LoadPhases(planningDir)
		if err != nil {
			return fmt.Errorf("cannot load phases: %w", err)
		}

		disp := display.New()

		phase, plan := state.FindNextPlan(phases)
		if plan == nil {
			disp.AllComplete()
			fmt.Println("\nTo add more work:")
			fmt.Println("  ralph add-phase \"New feature description\"")
			fmt.Println("  ralph plan N")
			return nil
		}

		// Create executor
		config := executor.DefaultConfig(cwd)
		if runModel != "" {
			config.Model = runModel
		}
		if maxRetries > 0 {
			config.MaxRetries = maxRetries
		} else if runLoop > 0 {
			// Default max-retries to same as loop value if not specified
			config.MaxRetries = runLoop
		}
		exec := executor.New(config)

		ctx := context.Background()

		if runLoop > 0 {
			// Autonomous loop mode
			return exec.LoopWithAnalysis(ctx, runLoop, runSkipAnalysis)
		}

		// Single plan execution
		result := exec.ExecutePlan(ctx, phase, plan)
		if !result.Success {
			return result.Error
		}

		// Run post-analysis to check observations and update subsequent plans
		analysisResult := exec.RunPostAnalysis(ctx, phase, plan, runSkipAnalysis)
		if analysisResult.Error != nil {
			disp.Warning(fmt.Sprintf("Post-analysis failed: %v", analysisResult.Error))
		} else if analysisResult.ObservationsFound > 0 {
			disp.Info("Analysis", fmt.Sprintf("%d observations analyzed", analysisResult.ObservationsFound))
		}

		// Show what's next
		fmt.Println()
		phases, _ = state.LoadPhases(planningDir)
		_, nextPlan := state.FindNextPlan(phases)
		if nextPlan != nil {
			disp.Info("Next", nextPlan.Name)
			fmt.Println("Run 'ralph run' to continue, or 'ralph run --loop' for autonomous execution.")
		} else {
			disp.Success("All plans in this phase complete!")
			fmt.Println("Run 'ralph status' to see overall progress.")
		}

		return nil
	},
}

func init() {
	runCmd.Flags().IntVarP(&runLoop, "loop", "l", 0, "run autonomous loop for N iterations (e.g., --loop 10)")
	runCmd.Flags().StringVar(&runModel, "model", "", "model to use (sonnet, opus, haiku)")
	runCmd.Flags().BoolVar(&runSkipAnalysis, "skip-analysis", false, "skip post-run observation analysis")
	runCmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Max retry attempts per plan (default: same as --loop value)")
	rootCmd.AddCommand(runCmd)
}
