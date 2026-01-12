package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var addPhaseCmd = &cobra.Command{
	Use:   "add-phase [description]",
	Short: "Add a new phase to the end of the roadmap",
	Long: `Add a new phase to the end of your project roadmap.

Requires: ROADMAP.md (run 'ralph roadmap' first)

Example:
  ralph add-phase "User notifications system"
  ralph add-phase "Performance optimization"

The new phase is added as the next numbered phase.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := strings.Join(args, " ")
		if description == "" {
			return fmt.Errorf("phase description required")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		return gsd.AddPhase(context.Background(), description)
	},
}

func init() {
	rootCmd.AddCommand(addPhaseCmd)
}
