package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/daydemir/ralph/internal/logs"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var logsListAll bool

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Manage verbatim conversation logs",
	Long: `Extract and manage Claude Code conversation logs.

Ralph can extract conversations from Claude Code's internal logs
and save them as readable markdown files in .planning/verbatim/

Subcommands:
  sync     Extract latest session(s) to .planning/verbatim/
  list     Show available sessions`,
}

var logsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Extract conversation logs from Claude Code",
	Long: `Extract Claude Code session logs to .planning/verbatim/

Finds the Claude project folder for the current workspace and
extracts the latest session as a readable markdown file.

The output includes user messages and Claude's text responses
(tool calls are excluded for readability).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		planningDir := filepath.Join(cwd, ".planning")

		// Check if .planning exists
		if _, err := os.Stat(planningDir); os.IsNotExist(err) {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s No .planning directory found\n", yellow("!"))
			fmt.Println("\nRun 'ralph discuss' to initialize your project first.")
			return nil
		}

		// Create extractor
		extractor, err := logs.NewVerbatimExtractor(cwd, planningDir)
		if err != nil {
			return fmt.Errorf("cannot initialize log extractor: %w", err)
		}

		green := color.New(color.FgGreen).SprintFunc()
		cyan := color.New(color.FgCyan).SprintFunc()

		fmt.Printf("Claude project folder: %s\n\n", cyan(extractor.ClaudeProjectPath()))

		// Extract latest session
		outPath, err := extractor.ExtractLatest()
		if err != nil {
			return fmt.Errorf("cannot extract session: %w", err)
		}

		fmt.Printf("%s Extracted to: %s\n", green("✓"), outPath)
		return nil
	},
}

var logsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available Claude sessions",
	Long:  `List all available Claude Code sessions for this project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		planningDir := filepath.Join(cwd, ".planning")

		// Create extractor
		extractor, err := logs.NewVerbatimExtractor(cwd, planningDir)
		if err != nil {
			return fmt.Errorf("cannot initialize log extractor: %w", err)
		}

		sessions, err := extractor.GetSessions()
		if err != nil {
			return fmt.Errorf("cannot get sessions: %w", err)
		}

		cyan := color.New(color.FgCyan).SprintFunc()
		dim := color.New(color.FgHiBlack).SprintFunc()

		fmt.Printf("Claude project folder: %s\n\n", cyan(extractor.ClaudeProjectPath()))

		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		fmt.Printf("Found %d session(s):\n\n", len(sessions))

		// Show most recent sessions (or all if --all flag)
		showCount := 10
		if logsListAll {
			showCount = len(sessions)
		}

		startIdx := len(sessions) - showCount
		if startIdx < 0 {
			startIdx = 0
		}

		for i := startIdx; i < len(sessions); i++ {
			s := sessions[i]
			shortID := s.ID
			if len(shortID) > 8 {
				shortID = shortID[:8] + "..."
			}
			fmt.Printf("  %s  %s  %s\n",
				s.EndTime.Format("2006-01-02 15:04"),
				cyan(shortID),
				dim(s.Path))
		}

		if !logsListAll && len(sessions) > showCount {
			fmt.Printf("\n  ... and %d more (use --all to show all)\n", len(sessions)-showCount)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsSyncCmd)
	logsCmd.AddCommand(logsListCmd)

	logsListCmd.Flags().BoolVarP(&logsListAll, "all", "a", false, "Show all sessions")
}
