package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/daydemir/ralph/internal/display"
	"github.com/daydemir/ralph/internal/executor"
	"github.com/daydemir/ralph/internal/planner"
	"github.com/daydemir/ralph/internal/state"
	"github.com/daydemir/ralph/internal/types"
	"github.com/daydemir/ralph/internal/utils"
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

		p := planner.NewPlanner("", cwd)

		// Check for required files
		if !p.HasProject() {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s No project found\n\n", yellow("!"))
			fmt.Println("Run 'ralph init' to create your project.")
			return nil
		}

		if !p.HasRoadmap() {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s Project found but no roadmap\n\n", yellow("!"))
			fmt.Println("Run 'ralph roadmap' to create your phase breakdown.")
			return nil
		}

		planningDir := p.PlanningDir()

		// Load state from JSON
		projectState, err := state.LoadStateJSON(planningDir)
		if err != nil {
			// state.json might not exist yet - show error and exit
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s Cannot load state: %v\n", yellow("!"), err)
			fmt.Println("\nState file not found or invalid.")
			return nil
		}

		// Load roadmap for project name and total phases
		roadmap, err := state.LoadRoadmapJSON(planningDir)
		if err != nil {
			return fmt.Errorf("cannot load roadmap: %w", err)
		}

		// Convert roadmap phases to state.Phase for display compatibility
		phases := make([]state.Phase, len(roadmap.Phases))
		total := 0
		completed := 0
		for i, p := range roadmap.Phases {
			// Find phase directory by number prefix (handles name mismatches)
			phaseDir := state.FindPhaseDirByNumber(planningDir, p.Number)
			if phaseDir == "" {
				// Fall back to slugified name if no directory found
				phaseDir = filepath.Join(planningDir, "phases",
					fmt.Sprintf("%02d-%s", p.Number, utils.Slugify(p.Name)))
			}
			phases[i] = state.Phase{
				Number: p.Number,
				Name:   p.Name,
				Path:   phaseDir,
			}

			// Load JSON plans from phase directory
			jsonPlans, err := state.LoadAllPlansJSON(phaseDir)
			if err == nil {
				total += len(jsonPlans)
				// Convert types.Plan to state.Plan for display
				for _, jp := range jsonPlans {
					summaryPath := filepath.Join(phaseDir,
						fmt.Sprintf("%02d-%s-summary.json", p.Number, jp.PlanNumber))
					_, summaryExists := os.Stat(summaryPath)
					isComplete := jp.Status == "complete" || summaryExists == nil
					if isComplete {
						completed++
					}
					phases[i].Plans = append(phases[i].Plans, state.Plan{
						Number:      jp.PlanNumber,
						Name:        jp.Objective,
						Path:        filepath.Join(phaseDir, fmt.Sprintf("%02d-%s.json", p.Number, jp.PlanNumber)),
						Status:      string(jp.Status),
						IsCompleted: isComplete,
					})
				}
			}
		}

		// Find next plan using JSON roadmap
		nextPhaseData, nextPlanData, _ := state.FindNextPlanJSON(planningDir)

		// Convert to display-compatible structures if found
		var nextPhase *types.Phase
		var nextPlan *types.Plan
		if nextPhaseData != nil && nextPlanData != nil {
			nextPhase, nextPlan = executor.ConvertToExecutionStructs(planningDir, nextPhaseData, nextPlanData)
		}

		// Display status
		cyan := color.New(color.FgCyan).SprintFunc()
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()
		bold := color.New(color.Bold).SprintFunc()
		dim := color.New(color.FgHiBlack).SprintFunc()

		fmt.Printf("%s\n%s\n\n", bold(roadmap.ProjectName), dim(fmt.Sprintf("ralph v%s", Version)))

		// Project artifacts
		printArtifacts(p, phases, green, dim)

		// Progress bar
		if total > 0 {
			barWidth := 20
			bar := display.CreateProgressBar(completed, total, barWidth)
			percentage := int(float64(completed) / float64(total) * 100)
			fmt.Printf("Progress: [%s] %d%% (%d/%d plans)\n\n", bar, percentage, completed, total)
		}

		// Current position
		fmt.Println(bold("📍 Current Position:"))
		totalPhases := len(roadmap.Phases)
		if projectState.CurrentPhase > 0 {
			fmt.Printf("  Phase: %d of %d\n", projectState.CurrentPhase, totalPhases)
			if nextPlan != nil {
				fmt.Printf("  Plan:  %s (next)\n", nextPlan.Name)
			} else {
				fmt.Printf("  Plan:  All complete\n")
			}
			// Derive status from phases
			status := "In progress"
			if nextPlan == nil {
				status = "Phase complete"
			}
			fmt.Printf("  Status: %s\n", status)
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
		printTips(p, dim, bold)

		// Verbose mode: show all phases and plans with progress bars
		if statusVerbose && len(phases) > 0 {
			fmt.Println(bold("All Phases:"))
			fmt.Println()
			for _, phase := range phases {
				phaseComplete := 0
				phaseTotal := len(phase.Plans)
				for _, p := range phase.Plans {
					if p.IsCompleted {
						phaseComplete++
					}
				}

				// Progress bar for phase
				barWidth := 10
				bar := "[" + display.CreateProgressBar(phaseComplete, phaseTotal, barWidth) + "]"

				var statusIcon string
				if phaseComplete == phaseTotal && phaseTotal > 0 {
					statusIcon = green("✓")
				} else if phaseComplete > 0 {
					statusIcon = yellow("◐")
				} else {
					statusIcon = "○"
				}

				fmt.Printf("%s Phase %d: %s %s %d/%d plans\n", statusIcon, phase.Number, phase.Name, bar, phaseComplete, phaseTotal)

				for _, plan := range phase.Plans {
					planIcon := "○"
					if plan.IsCompleted {
						planIcon = green("✓")
					}
					fmt.Printf("    %s %s\n", planIcon, plan.Name)
				}
				fmt.Println()
			}
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "show detailed phase and plan information")
	rootCmd.AddCommand(statusCmd)
}

// printArtifacts shows what artifacts exist in the project
func printArtifacts(p *planner.Planner, phases []state.Phase, green, dim func(a ...interface{}) string) {
	fmt.Println(dim("📦 Project Artifacts:"))

	// project.json
	if p.HasProject() {
		projectDesc := dim("Project vision and requirements")
		fmt.Printf("  %s project.json          %s\n", green("✓"), projectDesc)
	} else {
		fmt.Printf("  %s project.json          %s\n", "○", dim("Not created"))
	}

	// roadmap.json
	if p.HasRoadmap() {
		phaseCount := len(phases)
		roadmapDesc := dim(fmt.Sprintf("%d phases defined", phaseCount))
		fmt.Printf("  %s roadmap.json          %s\n", green("✓"), roadmapDesc)
	} else {
		fmt.Printf("  %s roadmap.json          %s\n", "○", dim("Not created"))
	}

	// Codebase maps
	if p.HasCodebaseMaps() {
		mapCount := countCodebaseMaps(p)
		mapsDesc := dim(fmt.Sprintf("%d analysis documents", mapCount))
		fmt.Printf("  %s Codebase Maps       %s\n", green("✓"), mapsDesc)
	} else {
		fmt.Printf("  %s Codebase Maps       %s\n", "○", dim("Not analyzed yet"))
	}

	// state.json
	if p.HasState() {
		fmt.Printf("  %s state.json            %s\n", green("✓"), dim("Tracking execution"))
	} else {
		fmt.Printf("  %s state.json            %s\n", "○", dim("Not started"))
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
func printSuggestedActions(phases []state.Phase, nextPlan *types.Plan, total, completed int, cyan, green, dim, bold func(a ...interface{}) string) {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
	fmt.Println(bold("🎯 Suggested Next Actions:"))
	fmt.Println()

	if nextPlan != nil {
		// Has incomplete plans - suggest execution
		fmt.Println("  Continue execution:")
		fmt.Printf("    %-30s %s\n", cyan("ralph run"), dim("Execute next incomplete plan"))
		fmt.Printf("    %-30s %s\n", cyan("ralph run --loop 5"), dim("Execute up to 5 plans autonomously"))
		fmt.Println()

		// Suggest planning ahead
		nextPhaseWithoutPlans := findNextPhaseWithoutPlans(phases)
		if nextPhaseWithoutPlans != nil {
			fmt.Println("  Plan ahead:")
			fmt.Printf("    %-30s %s\n", cyan("ralph discuss"), dim("Ralph will guide planning next phase"))
			fmt.Println()
		}

		fmt.Println("  Review progress:")
		fmt.Printf("    %-30s %s\n", cyan("ralph status -v"), dim("View all phases and plans"))

	} else if total == 0 {
		// No plans yet - suggest discuss to get started
		fmt.Println("  Get started:")
		fmt.Printf("    %-30s %s\n", cyan("ralph discuss"), dim("Ralph will help create your roadmap and plans"))

	} else {
		// All plans in existing phases are complete
		phaseNeedingPlans := findNextPhaseWithoutPlans(phases)
		if phaseNeedingPlans != nil {
			// Phase exists but has no plans - suggest discuss
			lastCompletedPhase := 0
			for _, p := range phases {
				if p.IsCompleted && p.Number > lastCompletedPhase {
					lastCompletedPhase = p.Number
				}
			}
			fmt.Printf("  %s Phase %d complete! Continue with:\n", green("✓"), lastCompletedPhase)
			fmt.Printf("    %-30s %s\n", cyan("ralph discuss"), dim("Ralph will plan the next phase"))
		} else {
			// Truly all complete!
			fmt.Printf("  %s All work complete! Consider:\n", green("✓"))
			fmt.Printf("    %-30s %s\n", cyan("ralph discuss \"add new feature\""), dim("Discuss adding more work"))
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

	fmt.Println(dim("Core Commands:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph discuss"), "Plan, update, review - Ralph decides context")
	fmt.Printf("  %-32s %s\n", cyan("ralph discuss \"context\""), "Discuss with specific context")
	fmt.Printf("  %-32s %s\n", cyan("ralph run"), "Execute next incomplete plan")
	fmt.Printf("  %-32s %s\n", cyan("ralph run --loop [N]"), "Execute up to N plans autonomously")
	fmt.Printf("  %-32s %s\n", cyan("ralph status"), "Show this dashboard")
	fmt.Printf("  %-32s %s\n", cyan("ralph status -v"), "List all phases and plans")
	fmt.Println()

	fmt.Println(dim("Advanced:"))
	fmt.Printf("  %-32s %s\n", cyan("ralph run --model opus"), "Use Claude Opus for complex plans")
	fmt.Printf("  %-32s %s\n", cyan("ralph --help"), "Show detailed help")
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
}

// printTips shows helpful context about Ralph workflow
func printTips(p *planner.Planner, dim, bold func(a ...interface{}) string) {
	fmt.Println(bold("💡 Tips:"))
	fmt.Println()

	fmt.Println(dim("• Three commands: discuss, run, status"))
	fmt.Println(dim("  'ralph discuss' handles all planning contextually"))
	fmt.Println()

	fmt.Println(dim("• Plans are stored in .planning/phases/NN-name/*.json"))
	fmt.Println(dim("  Ralph creates plans automatically based on your roadmap"))
	fmt.Println()

	fmt.Println(dim("• One roadmap per project (roadmap.json)"))
	fmt.Println(dim("  Use separate directories for multiple projects"))
	fmt.Println()
}

// countCodebaseMaps counts the number of codebase map files that exist
func countCodebaseMaps(p *planner.Planner) int {
	expectedMaps := []string{
		"ARCHITECTURE.md", "CONCERNS.md", "CONVENTIONS.md",
		"INTEGRATIONS.md", "STACK.md", "STRUCTURE.md", "TESTING.md",
	}
	count := 0
	codebaseDir := filepath.Join(p.PlanningDir(), "codebase")
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
