package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/daydemir/ralph/internal/config"
	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/prd"
	"github.com/daydemir/ralph/internal/prompts"
	"github.com/daydemir/ralph/internal/workspace"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	buildLoop       bool
	buildIterations int
	buildLLMBackend string
)

var buildCmd = &cobra.Command{
	Use:   "build [prd-id]",
	Short: "Execute PRDs from the backlog",
	Long: `Execute PRDs from the product backlog.

Examples:
  ralph build              Interactive: select a PRD to build
  ralph build auth-feature Build a specific PRD
  ralph build --loop       Autonomous loop until all PRDs complete
  ralph build --loop 5     Autonomous loop with max 5 iterations`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsDir, err := workspace.Find()
		if err != nil {
			return err
		}

		cfg, err := config.Load(wsDir)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var prdID string
		if len(args) > 0 {
			prdID = args[0]
		}

		if buildLoop {
			return runLoop(wsDir, cfg, buildIterations, buildLLMBackend)
		}

		return runSingle(wsDir, cfg, prdID, buildLLMBackend)
	},
}

func init() {
	buildCmd.Flags().BoolVar(&buildLoop, "loop", false, "run autonomous loop until complete")
	buildCmd.Flags().IntVarP(&buildIterations, "iterations", "n", 0, "max iterations for loop (0 = use config default)")
	buildCmd.Flags().StringVar(&buildLLMBackend, "llm", "claude", "LLM backend to use (claude, mistral)")
	rootCmd.AddCommand(buildCmd)
}

func runSingle(wsDir string, cfg *config.Config, prdID string, llmBackend string) error {
	// Load PRDs
	prdFile, err := prd.Load(workspace.PRDPath(wsDir))
	if err != nil {
		return fmt.Errorf("failed to load prd.json: %w", err)
	}

	pending := prdFile.Pending()
	if len(pending) == 0 {
		fmt.Println("All PRDs complete!")
		return nil
	}

	// If no PRD specified, show picker
	if prdID == "" {
		fmt.Println("Pending PRDs:")
		for i, p := range pending {
			fmt.Printf("  %d. %s - %s\n", i+1, p.ID, p.Description)
		}
		fmt.Println()
		fmt.Print("Select PRD (number or ID): ")

		var input string
		fmt.Scanln(&input)

		// Try as number first
		var idx int
		if _, err := fmt.Sscanf(input, "%d", &idx); err == nil && idx > 0 && idx <= len(pending) {
			prdID = pending[idx-1].ID
		} else {
			prdID = input
		}
	}

	// Verify PRD exists
	feature := prdFile.FindByID(prdID)
	if feature == nil {
		return fmt.Errorf("PRD not found: %s", prdID)
	}
	if feature.Passes {
		return fmt.Errorf("PRD already completed: %s", prdID)
	}

	fmt.Printf("Building PRD: %s\n", prdID)
	return executeIteration(wsDir, cfg, prdID, llmBackend)
}

func runLoop(wsDir string, cfg *config.Config, maxIterations int, llmBackend string) error {
	if maxIterations <= 0 {
		maxIterations = cfg.Build.DefaultLoopIterations
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt, finishing current iteration...")
		cancel()
	}()

	for i := 1; i <= maxIterations; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("Loop interrupted")
			return nil
		default:
		}

		// Check remaining PRDs
		prdFile, err := prd.Load(workspace.PRDPath(wsDir))
		if err != nil {
			return fmt.Errorf("failed to load prd.json: %w", err)
		}

		pending := prdFile.Pending()
		if len(pending) == 0 {
			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			fmt.Println(green("All PRDs complete!"))
			return nil
		}

		// Print iteration header
		cyan := color.New(color.FgCyan).SprintFunc()
		fmt.Println()
		fmt.Printf("=== %s ===\n", cyan(fmt.Sprintf("Ralph iteration %d of %d (started %s)", i, maxIterations, time.Now().Format("15:04:05"))))
		fmt.Println()

		// Show remaining PRDs
		fmt.Printf("PRDs (%d remaining):\n", len(pending))
		for _, p := range pending {
			fmt.Printf("  ○ %s\n", p.ID)
		}
		fmt.Println()

		// Execute iteration
		complete, err := executeIterationWithSignal(ctx, wsDir, cfg, "", llmBackend)
		if err != nil {
			fmt.Printf("Error in iteration %d: %v\n", i, err)
			continue
		}

		if complete {
			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			fmt.Printf("\n=== %s ===\n", green(fmt.Sprintf("Ralph complete after %d iterations", i)))
			return nil
		}

		fmt.Printf("\n=== Iteration %d complete ===\n", i)
	}

	yellow := color.New(color.FgYellow).SprintFunc()
	fmt.Printf("\n=== %s ===\n", yellow(fmt.Sprintf("Ralph stopped at max iterations (%d)", maxIterations)))
	return nil
}

func executeIteration(wsDir string, cfg *config.Config, prdID string, llmBackend string) error {
	_, err := executeIterationWithSignal(context.Background(), wsDir, cfg, prdID, llmBackend)
	return err
}

func executeIterationWithSignal(ctx context.Context, wsDir string, cfg *config.Config, prdID string, llmBackend string) (bool, error) {
	// Load prompt
	prompt, err := prompts.GetForWorkspace(wsDir, "build")
	if err != nil {
		return false, fmt.Errorf("failed to load build prompt: %w", err)
	}

	// If specific PRD, prepend to prompt
	if prdID != "" {
		prompt = fmt.Sprintf("Execute PRD: %s\n\n%s", prdID, prompt)
	}

	// Set up context files
	ralphDir := filepath.Join(wsDir, ".ralph")
	contextFiles := []string{
		filepath.Join(ralphDir, "prd.json"),
		filepath.Join(ralphDir, "progress.txt"),
		filepath.Join(ralphDir, "fix_plan.md"),
		filepath.Join(ralphDir, "codebase-map.md"),
	}

	// Create backend based on llmBackend flag
	var backend llm.Backend
	var executeErr string

	switch llmBackend {
	case "mistral":
		backend = llm.NewKiloCode(cfg.Mistral.Binary, cfg.Mistral.APIKey)
		executeErr = "failed to execute mistral"
	case "claude":
		fallthrough
	default:
		backend = llm.NewClaude(cfg.Claude.Binary)
		executeErr = "failed to execute claude"
	}

	// Execute
	opts := llm.ExecuteOptions{
		Prompt:       prompt,
		ContextFiles: contextFiles,
		Model:        cfg.LLM.Model,
		AllowedTools: cfg.Claude.AllowedTools,
		WorkDir:      wsDir,
	}

	reader, err := backend.Execute(ctx, opts)
	if err != nil {
		return false, fmt.Errorf("%s: %w", executeErr, err)
	}
	defer reader.Close()

	// Parse output
	handler := llm.NewConsoleHandler()
	if err := llm.ParseStream(reader, handler); err != nil {
		return false, fmt.Errorf("error parsing output: %w", err)
	}

	return handler.IsRalphComplete(), nil
}
