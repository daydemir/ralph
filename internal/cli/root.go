package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "ralph",
	Short: "Autonomous PRD execution loop for Claude Code",
	Long: `Ralph is a CLI tool for autonomous code development using Claude Code.

Two main modes:
  plan   - Create and modify PRDs interactively
  build  - Execute PRDs (single or autonomous loop)

Get started:
  ralph init              Initialize a new workspace
  ralph plan              Start planning mode
  ralph build [prd-id]    Build a specific PRD
  ralph build --loop      Autonomous execution loop`,
	Version: version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .ralph/config.yaml)")
	rootCmd.SetVersionTemplate(fmt.Sprintf("ralph version %s\n", version))
}

func exitError(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
