package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/daydemir/ralph/internal/state"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all phases and plans",
	Long: `List all phases and their plans with completion status.

Shows:
  ✓ Completed plans (have SUMMARY.md)
  ○ Pending plans (no SUMMARY.md yet)

Progress bars show completion status for each phase.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)

		if !gsd.HasRoadmap() {
			return fmt.Errorf("no roadmap found\n\nRun 'ralph roadmap' first to create your phase breakdown")
		}

		planningDir := gsd.PlanningDir()
		phases, err := state.LoadPhases(planningDir)
		if err != nil {
			return fmt.Errorf("cannot load phases: %w", err)
		}

		if len(phases) == 0 {
			fmt.Println("No phases found in .planning/phases/")
			fmt.Println("\nRun 'ralph roadmap' to create your phase breakdown.")
			return nil
		}

		// Load roadmap for project name
		roadmap, err := state.LoadRoadmapJSON(planningDir)
		projectName := "Project"
		if err == nil {
			projectName = roadmap.ProjectName
		}

		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()
		bold := color.New(color.Bold).SprintFunc()

		fmt.Println(bold(projectName))
		fmt.Println()

		for _, phase := range phases {
			// Count completed plans
			completed := 0
			for _, p := range phase.Plans {
				if p.IsCompleted {
					completed++
				}
			}
			total := len(phase.Plans)

			// Progress bar
			var bar string
			if total > 0 {
				progress := float64(completed) / float64(total)
				barWidth := 10
				filledWidth := int(progress * float64(barWidth))
				bar = "[" + strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth) + "]"
			} else {
				bar = "[          ]"
			}

			// Phase status icon
			var statusIcon string
			if completed == total && total > 0 {
				statusIcon = green("✓")
			} else if completed > 0 {
				statusIcon = yellow("◐")
			} else {
				statusIcon = "○"
			}

			// Extract readable phase name
			phaseName := phase.Name
			if idx := strings.Index(phaseName, "-"); idx > 0 {
				phaseName = phaseName[idx+1:]
			}

			fmt.Printf("%s Phase %d: %s %s %d/%d plans\n",
				statusIcon, phase.Number, phaseName, bar, completed, total)

			// List plans
			for _, plan := range phase.Plans {
				planIcon := "○"
				if plan.IsCompleted {
					planIcon = green("✓")
				}

				// Extract plan name
				planName := plan.Name
				fmt.Printf("    %s %s\n", planIcon, planName)
			}

			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
