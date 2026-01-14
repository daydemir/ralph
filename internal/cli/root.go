package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.4.0-alpha.3"
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "ralph",
	Short: "Autonomous plan execution with GSD planning",
	Long: `Ralph is an autonomous execution engine built on Get Shit Done (GSD) planning.

Planning Commands:
  init                Initialize a new project
  roadmap             Create phase breakdown
  map                 Analyze existing codebase
  discover [N]        Research a phase
  discuss [N]         Discuss a phase
  plan [N]            Create executable plans for a phase

Execution Commands:
  run                 Execute the next incomplete plan
  run --loop [N]      Autonomous loop (up to N plans)
  status              Show current position

Roadmap Modification:
  add-phase "desc"         Add phase to end
  insert-phase N "desc"    Insert as N.1
  remove-phase N           Remove and renumber

Workflow:
  1. ralph init              # Create PROJECT.md
  2. ralph map               # Analyze codebase (brownfield)
  3. ralph roadmap           # Create ROADMAP.md
  4. ralph plan 1            # Create plans for Phase 1
  5. ralph run               # Execute plans
  6. ralph run --loop 5      # Autonomous execution`,
	Version: version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .planning/config.json)")
	rootCmd.SetVersionTemplate(fmt.Sprintf("ralph version %s\n", version))
}

func exitError(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
