package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set by goreleaser via ldflags
	Version = "dev"
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "ralph",
	Short: "Autonomous plan execution with intelligent planning",
	Long: `Ralph is an autonomous execution engine with intelligent planning.

Core Commands:
  discuss [context]   Plan, review, update - Ralph determines context
  run                 Execute the next incomplete plan
  run --loop [N]      Autonomous execution (up to N plans)
  status              Show current position and progress
  status -v           Show all phases and plans

How ralph discuss works:
  No project?    → Start project initialization
  No roadmap?    → Create your phase breakdown
  No plans?      → Create plans for current phase
  Has plans?     → Review plans before execution
  "context"      → Incorporate your input into the discussion

Workflow:
  1. ralph discuss          # Creates project and roadmap
  2. ralph discuss          # Creates plans for Phase 1
  3. ralph run              # Execute plans
  4. ralph run --loop 5     # Autonomous execution`,
	Version: Version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .planning/config.json)")
	rootCmd.SetVersionTemplate(fmt.Sprintf("ralph version %s\n", Version))
}

func exitError(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
