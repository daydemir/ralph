package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/daydemir/ralph/internal/planner"
	"github.com/spf13/cobra"
)

var insertPhaseCmd = &cobra.Command{
	Use:   "insert-phase [after-phase] [description]",
	Short: "Insert urgent work between existing phases",
	Long: `Insert a new phase between existing phases.

Requires: ROADMAP.md (run 'ralph roadmap' first)

Example:
  ralph insert-phase 3 "Critical security patch"

This creates Phase 3.1 between Phase 3 and Phase 4.
Other phases don't renumber - you can have 3.1, 3.2, etc.

Great for urgent work that can't wait until the next full phase.`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		afterPhase, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid phase number: %s", args[0])
		}

		description := strings.Join(args[1:], " ")
		if description == "" {
			return fmt.Errorf("phase description required")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gsd := planner.NewGSD("", cwd)
		return gsd.InsertPhase(context.Background(), afterPhase, description)
	},
}

func init() {
	rootCmd.AddCommand(insertPhaseCmd)
}
