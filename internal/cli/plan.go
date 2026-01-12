package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/daydemir/ralph/internal/config"
	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/prompts"
	"github.com/daydemir/ralph/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	planLLM string
	planCmd = &cobra.Command{
		Use:   "plan",
		Short: "Interactive planning mode for creating PRDs",
		Long: `Start an interactive planning session with Claude.

In planning mode, you can:
  - Discuss what features to build
  - Have Claude research your codebase
  - Create well-defined PRDs
  - Add PRDs to the backlog

This opens an interactive Claude Code session.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			wsDir, err := workspace.Find()
			if err != nil {
				return err
			}

			cfg, err := config.Load(wsDir)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			return runPlan(wsDir, cfg, planLLM)
		},
	}
)

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVar(&planLLM, "llm", "claude", "LLM backend to use (claude, mistral)")
}

func runPlan(wsDir string, cfg *config.Config, llmBackend string) error {
	// Load the plan prompt
	prompt, err := prompts.GetForWorkspace(wsDir, "plan")
	if err != nil {
		return fmt.Errorf("failed to load plan prompt: %w", err)
	}

	// Set up context files
	ralphDir := filepath.Join(wsDir, ".ralph")
	contextFiles := []string{
		filepath.Join(ralphDir, "prd.json"),
		filepath.Join(ralphDir, "codebase-map.md"),
		filepath.Join(ralphDir, "progress.txt"),
	}

	// Create backend based on flag
	var backend llm.Backend
	switch llmBackend {
	case "mistral":
		backend = llm.NewKiloCode(cfg.Mistral.Binary, cfg.Mistral.APIKey)
	case "claude":
		fallthrough
	default:
		backend = llm.NewClaude(cfg.Claude.Binary)
	}

	fmt.Println("Starting planning mode...")
	fmt.Println("Use this session to discuss features and create PRDs.")
	fmt.Println()

	// Execute interactively
	opts := llm.ExecuteOptions{
		Prompt:       prompt,
		ContextFiles: contextFiles,
		Model:        cfg.LLM.Model,
		AllowedTools: cfg.Claude.AllowedTools,
		WorkDir:      wsDir,
	}

	return backend.ExecuteInteractive(context.Background(), opts)
}
