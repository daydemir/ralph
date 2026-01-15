package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
		dim := color.New(color.FgHiBlack).SprintFunc()

		fmt.Printf("%s\n%s\n\n", bold(st.ProjectName), dim(fmt.Sprintf("ralph v%s", Version)))

		// Project artifacts
		printArtifacts(gsd, phases, green, dim)

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
		fmt.Println(bold("📍 Current Position:"))
		if st.CurrentPhase > 0 {
			fmt.Printf("  Phase: %d of %d\n", st.CurrentPhase, st.TotalPhases)
			if nextPlan != nil {
				fmt.Printf("  Plan:  %s (next)\n", nextPlan.Name)
			} else {
				fmt.Printf("  Plan:  All complete\n")
			}
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

		// Suggested actions (context-aware)
		printSuggestedActions(phases, nextPlan, total, completed, cyan, green, dim, bold)

		// Available commands
		printAvailableCommands(cyan, dim, bold)

		// Tips
		printTips(gsd, dim, bold)

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

// printArtifacts shows what artifacts exist in the project
func printArtifacts(gsd *planner.GSD, phases []state.Phase, green, dim func(a ...interface{}) string) {
	fmt.Println(dim("📦 Project Artifacts:"))

	// PROJECT.md
	if gsd.HasProject() {
		projectDesc := dim("Project vision and requirements")
		fmt.Printf("  %s PROJECT.md          %s\n", green("✓"), projectDesc)
	} else {
		fmt.Printf("  %s PROJECT.md          %s\n", "○", dim("Not created"))
	}

	// ROADMAP.md
	if gsd.HasRoadmap() {
		phaseCount := len(phases)
		roadmapDesc := dim(fmt.Sprintf("%d phases defined", phaseCount))
		fmt.Printf("  %s ROADMAP.md          %s\n", green("✓"), roadmapDesc)
	} else {
		fmt.Printf("  %s ROADMAP.md          %s\n", "○", dim("Not created"))
	}

	// Codebase maps
	if gsd.HasCodebaseMaps() {
		mapCount := countCodebaseMaps(gsd)
		mapsDesc := dim(fmt.Sprintf("%d analysis documents", mapCount))
		fmt.Printf("  %s Codebase Maps       %s\n", green("✓"), mapsDesc)
	} else {
		fmt.Printf("  %s Codebase Maps       %s\n", "○", dim("Not analyzed yet"))
	}

	// STATE.md
	if gsd.HasState() {
		fmt.Printf("  %s STATE.md            %s\n", green("✓"), dim("Tracking execution"))
	} else {
		fmt.Printf("  %s STATE.md            %s\n", "○", dim("Not started"))
	}

	// Plans
	phaseCount := len(phases)
	phasesWithPlans := countPhasesWithPlans(phases)
	if phasesWithPlans > 0 {
		plansDesc := dim(fmt.Sprintf("%d/%d phases have plans", phasesWithPlans, phaseCount))
		fmt.Printf("  %s Plans               %s\n", green("✓"), plansDesc)
	} else if phaseCount > 0 {
		fmt.Printf("  %s Plans               %s\n", "○", dim("No phases planned yet"))
	}

	fmt.Println()
}

// printSuggestedActions shows context-aware next steps
func printSuggestedActions(phases []state.Phase, nextPlan *state.Plan, total, completed int, cyan, green, dim, bold func(a ...interface{}) string) {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
	fmt.Println(bold("🎯 Suggested Next Actions:"))
	fmt.Println()

	if nextPlan != nil {
		// Check if there's a phase ready for review (has plans but none executed)
		phaseReadyForReview := findPhaseReadyForReview(phases)
		if phaseReadyForReview != nil {
			fmt.Println("  Review before running:")
			reviewCmd := fmt.Sprintf("ralph review %d", phaseReadyForReview.Number)
			fmt.Printf("    %-30s %s\n", cyan(reviewCmd), dim("Walk through plans before first execution"))
			fmt.Println()
		}

		// Has incomplete plans - suggest execution
		fmt.Println("  Continue execution:")
		fmt.Printf("    %-30s %s\n", cyan("ralph run"), dim("Execute next incomplete plan"))
		fmt.Printf("    %-30s %s\n", cyan("ralph run --loop 5"), dim("Execute up to 5 plans autonomously"))
		fmt.Println()

		// Suggest planning ahead
		nextPhaseWithoutPlans := findNextPhaseWithoutPlans(phases)
		if nextPhaseWithoutPlans != nil {
			fmt.Println("  Plan ahead:")
			cmd := fmt.Sprintf("ralph plan %d", nextPhaseWithoutPlans.Number)
			desc := fmt.Sprintf("Create plans for Phase %d", nextPhaseWithoutPlans.Number)
			fmt.Printf("    %-30s %s\n", cyan(cmd), dim(desc))

			discussCmd := fmt.Sprintf("ralph discuss %d", nextPhaseWithoutPlans.Number)
			discussDesc := fmt.Sprintf("Gather context before planning Phase %d", nextPhaseWithoutPlans.Number)
			fmt.Printf("    %-30s %s\n", cyan(discussCmd), dim(discussDesc))
			fmt.Println()
		}

		fmt.Println("  Review progress:")
		fmt.Printf("    %-30s %s\n", cyan("ralph list"), dim("View all phases and plans"))
		fmt.Printf("    %-30s %s\n", cyan("ralph status -v"), dim("Show detailed completion status"))

	} else if total == 0 {
		// No plans yet - suggest planning first phase
		fmt.Println("  Create your first plans:")
		fmt.Printf("    %-30s %s\n", cyan("ralph discover 1"), dim("Research Phase 1 approach"))
		fmt.Printf("    %-30s %s\n", cyan("ralph discuss 1"), dim("Discuss Phase 1 interactively"))
		fmt.Printf("    %-30s %s\n", cyan("ralph plan 1"), dim("Create executable plans for Phase 1"))

	} else {
		// All plans in existing phases are complete
		// Check if there are phases without plans (empty directories)
		phaseNeedingPlans := findNextPhaseWithoutPlans(phases)
		if phaseNeedingPlans != nil {
			// Phase exists but has no plans - suggest planning it
			// Find the last completed phase number
			lastCompletedPhase := 0
			for _, p := range phases {
				if p.IsCompleted && p.Number > lastCompletedPhase {
					lastCompletedPhase = p.Number
				}
			}
			fmt.Printf("  %s Phase %d complete! Next phase needs planning:\n", green("✓"), lastCompletedPhase)
			fmt.Printf("    %-30s %s\n", cyan(fmt.Sprintf("ralph discuss %d", phaseNeedingPlans.Number)),
				dim(fmt.Sprintf("Gather context for Phase %d", phaseNeedingPlans.Number)))
			fmt.Printf("    %-30s %s\n", cyan(fmt.Sprintf("ralph plan %d", phaseNeedingPlans.Number)),
				dim(fmt.Sprintf("Create plans for Phase %d", phaseNeedingPlans.Number)))
		} else {
			// Truly all complete!
			fmt.Printf("  %s All work complete! Consider:\n", green("✓"))
			fmt.Printf("    %-30s %s\n", cyan("ralph add-phase \"description\""), dim("Add more work to roadmap"))
			fmt.Printf("    %s\n", dim("Or ship your milestone and start a new project"))
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
}

// printAvailableCommands shows comprehensive command reference
func printAvailableCommands(cyan, dim, bold func(a ...interface{}) string) {
	fmt.Println(bold("🛠️  Available Commands:"))
	fmt.Println()

	fmt.Println(dim("Planning Workflow:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph init"), "Initialize new project (PROJECT.md)")
	fmt.Printf("  %-32s %s\n", cyan("ralph map"), "Analyze codebase structure (brownfield)")
	fmt.Printf("  %-32s %s\n", cyan("ralph roadmap"), "Create phase breakdown (ROADMAP.md)")
	fmt.Println()

	fmt.Println(dim("Phase Preparation:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph discover N"), "Research Phase N before planning")
	fmt.Printf("  %-32s %s\n", cyan("ralph discuss N"), "Gather context for Phase N interactively")
	fmt.Printf("  %-32s %s\n", cyan("ralph plan N"), "Create executable plans for Phase N")
	fmt.Printf("  %-32s %s\n", cyan("ralph review N"), "Review plans before execution")
	fmt.Println()

	fmt.Println(dim("Execution:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph run"), "Execute next incomplete plan")
	fmt.Printf("  %-32s %s\n", cyan("ralph run --loop [N]"), "Execute up to N plans (default: 5)")
	fmt.Printf("  %-32s %s\n", cyan("ralph status"), "Show this dashboard")
	fmt.Printf("  %-32s %s\n", cyan("ralph list"), "List all phases and plans")
	fmt.Println()

	fmt.Println(dim("Roadmap Management:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph update"), "Conversational roadmap updates")
	fmt.Printf("  %-32s %s\n", cyan("ralph add-phase \"desc\""), "Add new phase to end of roadmap")
	fmt.Printf("  %-32s %s\n", cyan("ralph insert-phase N \"desc\""), "Insert urgent work as Phase N.1")
	fmt.Printf("  %-32s %s\n", cyan("ralph remove-phase N"), "Remove phase and renumber")
	fmt.Println()

	fmt.Println(dim("Advanced:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph run --model opus"), "Use Claude Opus for complex plans")
	fmt.Printf("  %-32s %s\n", cyan("ralph --help"), "Show detailed help for all commands")
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
}

// printTips shows helpful context about Ralph workflow
func printTips(gsd *planner.GSD, dim, bold func(a ...interface{}) string) {
	fmt.Println(bold("💡 Tips:"))
	fmt.Println()

	fmt.Println(dim("• You have ONE roadmap per project (ROADMAP.md)"))
	fmt.Println(dim("  To work on multiple projects, use separate directories"))
	fmt.Println()

	fmt.Println(dim("• Plans are auto-generated from phases"))
	fmt.Println(dim("  'ralph plan N' creates PLAN.md files in .planning/phases/NN-name/"))
	fmt.Println()

	if !gsd.HasCodebaseMaps() {
		fmt.Println(dim("• Maps are optional but helpful for brownfield projects"))
		fmt.Println(dim("  'ralph map' creates 7 analysis docs in .planning/codebase/"))
		fmt.Println()
	}

	fmt.Println(dim("• Use 'discuss' before 'plan' for complex phases"))
	fmt.Println(dim("  Helps gather requirements and approach before creating plans"))
	fmt.Println()
}

// countCodebaseMaps counts the number of codebase map files that exist
func countCodebaseMaps(gsd *planner.GSD) int {
	expectedMaps := []string{
		"ARCHITECTURE.md", "CONCERNS.md", "CONVENTIONS.md",
		"INTEGRATIONS.md", "STACK.md", "STRUCTURE.md", "TESTING.md",
	}
	count := 0
	codebaseDir := filepath.Join(gsd.PlanningDir(), "codebase")
	for _, mapFile := range expectedMaps {
		if _, err := os.Stat(filepath.Join(codebaseDir, mapFile)); err == nil {
			count++
		}
	}
	return count
}

// countPhasesWithPlans counts how many phases have at least one plan
func countPhasesWithPlans(phases []state.Phase) int {
	count := 0
	for _, phase := range phases {
		if len(phase.Plans) > 0 {
			count++
		}
	}
	return count
}

// findNextPhaseWithoutPlans finds the first phase that doesn't have any plans
func findNextPhaseWithoutPlans(phases []state.Phase) *state.Phase {
	for i := range phases {
		if len(phases[i].Plans) == 0 {
			return &phases[i]
		}
	}
	return nil
}

// findPhaseReadyForReview finds a phase that has plans but none completed (fresh, ready for review)
func findPhaseReadyForReview(phases []state.Phase) *state.Phase {
	for i := range phases {
		if len(phases[i].Plans) > 0 {
			// Check if any plans are completed
			hasCompleted := false
			for _, p := range phases[i].Plans {
				if p.IsCompleted {
					hasCompleted = true
					break
				}
			}
			// Phase has plans but none completed - ready for review
			if !hasCompleted {
				return &phases[i]
			}
		}
	}
	return nil
}
