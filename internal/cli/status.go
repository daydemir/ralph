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

var statusVerbose bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current project position",
	Long: `Show your current position in the project roadmap.

Displays:
  - Current phase and plan
  - Recent activity
  - Progress metrics
  - Next recommended action

Use --verbose for detailed information including decisions and issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)

		// Check for required files
		if !gsd.HasProject() {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s No project found\n\n", yellow("!"))
			fmt.Println("Run 'ralph init' to create your project.")
			return nil
		}

		if !gsd.HasRoadmap() {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s Project found but no roadmap\n\n", yellow("!"))
			fmt.Println("Run 'ralph roadmap' to create your phase breakdown.")
			return nil
		}

		planningDir := gsd.PlanningDir()

		// Load state
		st, err := state.LoadState(planningDir)
		if err != nil {
			// STATE.md might not exist yet
			st = &state.State{
				ProjectName: "Project",
				Status:      "Not started",
			}
		}

		// Load phases
		phases, err := state.LoadPhases(planningDir)
		if err != nil {
			phases = []state.Phase{}
		}

		// Count plans
		total, completed := state.CountPlans(phases)

		// Find next plan
		nextPhase, nextPlan := state.FindNextPlan(phases)

		// Display status
		cyan := color.New(color.FgCyan).SprintFunc()
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()
		bold := color.New(color.Bold).SprintFunc()

		fmt.Printf("%s v%s - %s\n\n", bold("Ralph"), version, st.ProjectName)

		// Progress bar
		if total > 0 {
			progress := float64(completed) / float64(total)
			barWidth := 20
			filledWidth := int(progress * float64(barWidth))
			bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)
			percentage := int(progress * 100)
			fmt.Printf("Progress: [%s] %d%% (%d/%d plans)\n\n", bar, percentage, completed, total)
		}

		// Current position
		fmt.Println(bold("Current Position:"))
		if st.CurrentPhase > 0 {
			fmt.Printf("  Phase: %d of %d\n", st.CurrentPhase, st.TotalPhases)
			fmt.Printf("  Plan:  %d of %d\n", st.CurrentPlan, st.TotalPlans)
			fmt.Printf("  Status: %s\n", st.Status)
		} else if nextPhase != nil {
			fmt.Printf("  Phase: %d (%s)\n", nextPhase.Number, nextPhase.Name)
			fmt.Printf("  Plan:  %s\n", nextPlan.Name)
			fmt.Printf("  Status: Ready to execute\n")
		} else if total == 0 {
			fmt.Printf("  No plans created yet\n")
		} else {
			fmt.Printf("  Status: %s All plans complete!\n", green("✓"))
		}
		fmt.Println()

		// Next action
		fmt.Println(bold("Next Action:"))
		if total == 0 {
			fmt.Printf("  Run: %s\n", cyan("ralph plan 1"))
			fmt.Println("  Create plans for the first phase")
		} else if nextPlan != nil {
			fmt.Printf("  Run: %s\n", cyan("ralph run"))
			fmt.Printf("  Execute: %s\n", nextPlan.Name)
		} else {
			fmt.Printf("  %s All work complete! Consider:\n", green("✓"))
			fmt.Printf("    • %s to add more work\n", cyan("ralph add-phase \"description\""))
			fmt.Printf("    • Review and ship your milestone\n")
		}
		fmt.Println()

		// Verbose mode: show phases
		if statusVerbose && len(phases) > 0 {
			fmt.Println(bold("Phases:"))
			for _, phase := range phases {
				phaseComplete := 0
				phaseTotal := len(phase.Plans)
				for _, p := range phase.Plans {
					if p.IsCompleted {
						phaseComplete++
					}
				}

				var statusIcon string
				if phaseComplete == phaseTotal && phaseTotal > 0 {
					statusIcon = green("✓")
				} else if phaseComplete > 0 {
					statusIcon = yellow("◐")
				} else {
					statusIcon = "○"
				}

				fmt.Printf("  %s Phase %d: %s (%d/%d)\n", statusIcon, phase.Number, phase.Name, phaseComplete, phaseTotal)

				for _, plan := range phase.Plans {
					planIcon := "○"
					if plan.IsCompleted {
						planIcon = green("✓")
					}
					fmt.Printf("      %s %s\n", planIcon, plan.Name)
				}
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "show detailed phase and plan information")
	rootCmd.AddCommand(statusCmd)
}
