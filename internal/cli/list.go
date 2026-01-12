package cli

import (
	"fmt"

	"github.com/daydemir/ralph/internal/prd"
	"github.com/daydemir/ralph/internal/workspace"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listPending   bool
	listCompleted bool
	listJSON      bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List PRDs in the backlog",
	Long: `List PRDs with their status.

Examples:
  ralph list              Show all PRDs
  ralph list --pending    Show only pending PRDs
  ralph list --completed  Show only completed PRDs
  ralph list --json       Output as JSON`,
	RunE: func(cmd *cobra.Command, args []string) error {
		wsDir, err := workspace.Find()
		if err != nil {
			return err
		}

		prdFile, err := prd.Load(workspace.PRDPath(wsDir))
		if err != nil {
			return fmt.Errorf("failed to load prd.json: %w", err)
		}

		if listJSON {
			return prd.PrintJSON(prdFile, listPending, listCompleted)
		}

		return printList(prdFile)
	},
}

func init() {
	listCmd.Flags().BoolVar(&listPending, "pending", false, "show only pending PRDs")
	listCmd.Flags().BoolVar(&listCompleted, "completed", false, "show only completed PRDs")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)
}

func printList(prdFile *prd.File) error {
	pending := 0
	completed := 0

	for _, f := range prdFile.Features {
		if f.Passes {
			completed++
		} else {
			pending++
		}
	}

	fmt.Printf("PRDs (%d pending, %d completed):\n", pending, completed)

	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	for _, f := range prdFile.Features {
		if listPending && f.Passes {
			continue
		}
		if listCompleted && !f.Passes {
			continue
		}

		status := yellow("○")
		if f.Passes {
			status = green("✓")
		}

		fmt.Printf("  %s %-25s %s\n", status, f.ID, truncate(f.Description, 50))
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
