package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daydemir/ralph/internal/executor"
	"github.com/daydemir/ralph/internal/planner"
	"github.com/daydemir/ralph/internal/state"
	"github.com/spf13/cobra"
)

var digestCmd = &cobra.Command{
	Use:   "digest [plan-path]",
	Short: "Analyze discoveries from a completed plan",
	Long: `Run post-execution analysis on a completed plan's discoveries.

This command is automatically run after 'ralph run', but can be
invoked manually to re-analyze after manual fixes or to review
discoveries from a specific plan.

Examples:
  ralph digest                                    # Analyze most recently completed plan
  ralph digest .planning/phases/01-*/01-03-PLAN.md  # Analyze specific plan
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		if !gsd.HasRoadmap() {
			return fmt.Errorf("no ROADMAP.md found - run 'ralph roadmap' first")
		}

		planningDir := gsd.PlanningDir()

		// Load roadmap for phases
		roadmap, err := state.LoadRoadmapJSON(planningDir)
		if err != nil {
			return fmt.Errorf("cannot load roadmap: %w", err)
		}

		// Convert roadmap phases to state.Phase for compatibility
		phases := make([]state.Phase, len(roadmap.Phases))
		for i, p := range roadmap.Phases {
			phaseDir := filepath.Join(planningDir, "phases",
				fmt.Sprintf("%02d-%s", p.Number, slugify(p.Name)))
			phases[i] = state.Phase{
				Number: p.Number,
				Name:   p.Name,
				Path:   phaseDir,
			}
		}

		var targetPlan *state.Plan
		var targetPhase *state.Phase

		if len(args) > 0 {
			// Find the specified plan
			planPath := args[0]
			if !filepath.IsAbs(planPath) {
				planPath = filepath.Join(cwd, planPath)
			}

			for i := range phases {
				for j := range phases[i].Plans {
					if phases[i].Plans[j].Path == planPath {
						targetPlan = &phases[i].Plans[j]
						targetPhase = &phases[i]
						break
					}
				}
				if targetPlan != nil {
					break
				}
			}

			if targetPlan == nil {
				return fmt.Errorf("plan not found: %s", args[0])
			}
		} else {
			// Find most recently completed plan
			for i := len(phases) - 1; i >= 0; i-- {
				for j := len(phases[i].Plans) - 1; j >= 0; j-- {
					if phases[i].Plans[j].IsCompleted {
						targetPlan = &phases[i].Plans[j]
						targetPhase = &phases[i]
						break
					}
				}
				if targetPlan != nil {
					break
				}
			}

			if targetPlan == nil {
				fmt.Println("No completed plans found to analyze.")
				return nil
			}
		}

		fmt.Printf("Analyzing: %s\n\n", targetPlan.Name)

		// Read the plan content
		content, err := os.ReadFile(targetPlan.Path)
		if err != nil {
			return fmt.Errorf("cannot read plan: %w", err)
		}

		// Parse observations
		observations := executor.ParseObservations(string(content), nil)

		if len(observations) == 0 {
			fmt.Println("No observations found in this plan.")
			return nil
		}

		fmt.Printf("Found %d observations:\n\n", len(observations))
		for i, o := range observations {
			fmt.Printf("%d. [%s/%s] %s\n", i+1, o.Type, o.Severity, o.Title)
			fmt.Printf("   %s\n", o.Detail)
			if o.File != "" {
				fmt.Printf("   File: %s\n", o.File)
			}
			fmt.Printf("   Action: %s\n\n", o.Action)
		}

		// Check for actionable observations
		if !executor.HasActionableObservations(observations) {
			fmt.Println("No actionable observations (all are informational).")
			return nil
		}

		// Run full analysis
		fmt.Println("Running analysis to update subsequent plans...")

		config := executor.DefaultConfig(cwd)
		exec := executor.New(config)

		ctx := context.Background()
		result := exec.RunPostAnalysis(ctx, targetPhase, targetPlan, false)

		if result.Error != nil {
			return fmt.Errorf("analysis failed: %w", result.Error)
		}

		fmt.Println("Analysis complete.")
		if result.PlansModified > 0 {
			fmt.Printf("Modified %d subsequent plans.\n", result.PlansModified)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(digestCmd)
}
