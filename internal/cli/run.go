package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/daydemir/ralph/internal/executor"
	"github.com/daydemir/ralph/internal/planner"
	"github.com/daydemir/ralph/internal/state"
	"github.com/spf13/cobra"
)

var (
	runLoop       bool
	runLoopCount  int
	runModel      string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the next incomplete plan",
	Long: `Execute plans using Claude Code with automated verification.

Single execution (default):
  ralph run

  Executes the next incomplete plan and stops.

Autonomous loop:
  ralph run --loop
  ralph run --loop 5

  Runs multiple plans automatically until:
  - All plans complete
  - A verification fails
  - Max iterations reached (default 10)

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

		phase, plan := state.FindNextPlan(phases)
		if plan == nil {
			fmt.Println("All plans complete! No more work to do.")
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
		exec := executor.New(config)

		ctx := context.Background()

		if runLoop {
			// Autonomous loop mode
			maxIterations := runLoopCount
			if maxIterations <= 0 {
				maxIterations = 10
			}
			return exec.Loop(ctx, maxIterations)
		}

		// Single plan execution
		result := exec.ExecutePlan(ctx, phase, plan)
		if !result.Success {
			return result.Error
		}

		// Show what's next
		fmt.Println()
		phases, _ = state.LoadPhases(planningDir)
		_, nextPlan := state.FindNextPlan(phases)
		if nextPlan != nil {
			fmt.Printf("Next: %s\n", nextPlan.Name)
			fmt.Println("Run 'ralph run' to continue, or 'ralph run --loop' for autonomous execution.")
		} else {
			fmt.Println("All plans in this phase complete!")
			fmt.Println("Run 'ralph status' to see overall progress.")
		}

		return nil
	},
}

func init() {
	runCmd.Flags().BoolVar(&runLoop, "loop", false, "run autonomous loop")
	runCmd.Flags().IntVarP(&runLoopCount, "count", "n", 10, "max iterations for loop mode")
	runCmd.Flags().StringVar(&runModel, "model", "", "model to use (sonnet, opus, haiku)")
	rootCmd.AddCommand(runCmd)
}
