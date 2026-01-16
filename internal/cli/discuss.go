package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/daydemir/ralph/internal/state"
	"github.com/spf13/cobra"
)

var discussCmd = &cobra.Command{
	Use:   "discuss [context]",
	Short: "Plan, review, update - Ralph determines what to do",
	Long: `Start an interactive planning session with Claude.

Ralph inspects your current project state and starts the appropriate discussion:

  No project?       → Initialize a new project
  No roadmap?       → Create your phase breakdown
  No plans?         → Create plans for the current phase
  Has plans?        → Review plans or discuss updates

You can optionally provide context to guide the discussion:

  ralph discuss                    # Ralph decides based on state
  ralph discuss "need to add auth" # Ralph incorporates your context

This replaces the individual init, roadmap, plan, review, and update commands
with a single intelligent entry point.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		p := planner.NewPlanner("", cwd)
		ctx := context.Background()

		// Get optional user context
		var userContext string
		if len(args) > 0 {
			userContext = strings.TrimSpace(args[0])
		}

		// Determine what to do based on state
		if !p.HasProject() {
			// No project - initialize
			fmt.Println("Starting project initialization...")
			if userContext != "" {
				fmt.Printf("Context: %s\n\n", userContext)
			}
			return p.NewProject(ctx)
		}

		if !p.HasRoadmap() {
			// Has project but no roadmap - create roadmap
			fmt.Println("Creating project roadmap...")
			if userContext != "" {
				fmt.Printf("Context: %s\n\n", userContext)
			}
			return p.CreateRoadmap(ctx)
		}

		// Has roadmap - check for plans
		planningDir := p.PlanningDir()
		roadmap, err := state.LoadRoadmapJSON(planningDir)
		if err != nil {
			return fmt.Errorf("cannot load roadmap: %w", err)
		}

		// Find the first phase without plans or with incomplete plans
		for _, phase := range roadmap.Phases {
			phaseDir := state.FindPhaseDirByNumber(planningDir, phase.Number)
			if phaseDir == "" {
				// No directory for this phase yet - create plans
				fmt.Printf("Planning Phase %d: %s...\n", phase.Number, phase.Name)
				if userContext != "" {
					fmt.Printf("Context: %s\n\n", userContext)
				}
				return p.PlanPhase(ctx, fmt.Sprintf("%d", phase.Number))
			}

			// Check if this phase has plans
			plans, err := state.LoadAllPlansJSON(phaseDir)
			if err != nil || len(plans) == 0 {
				// No plans - create them
				fmt.Printf("Planning Phase %d: %s...\n", phase.Number, phase.Name)
				if userContext != "" {
					fmt.Printf("Context: %s\n\n", userContext)
				}
				return p.PlanPhase(ctx, fmt.Sprintf("%d", phase.Number))
			}

			// Check if all plans are complete
			allComplete := true
			for _, plan := range plans {
				if plan.Status != "complete" {
					allComplete = false
					break
				}
			}

			if !allComplete {
				// Has incomplete plans - offer to review or update
				if userContext != "" {
					// User has context - do an update discussion
					fmt.Printf("Discussing updates for Phase %d...\n", phase.Number)
					fmt.Printf("Context: %s\n\n", userContext)
					return p.UpdateRoadmap(ctx)
				}
				// No context - offer review
				fmt.Printf("Reviewing Phase %d plans before execution...\n", phase.Number)
				return p.ReviewPlans(ctx, fmt.Sprintf("%d", phase.Number))
			}
		}

		// All phases have complete plans
		if userContext != "" {
			// User wants to discuss something - update roadmap
			fmt.Println("Discussing roadmap updates...")
			fmt.Printf("Context: %s\n\n", userContext)
			return p.UpdateRoadmap(ctx)
		}

		// Everything is complete
		fmt.Println("All phases have plans and are complete!")
		fmt.Println("\nTo add more work, run:")
		fmt.Println("  ralph discuss \"description of new work\"")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(discussCmd)
}
