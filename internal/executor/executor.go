package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/state"
	"github.com/fatih/color"
)

// Config holds executor configuration
type Config struct {
	ClaudeBinary           string
	Model                  string
	InactivityTimeoutMins  int
	WorkDir                string
	PlanningDir            string
}

// DefaultConfig returns default executor configuration
func DefaultConfig(workDir string) *Config {
	return &Config{
		ClaudeBinary:          "claude",
		Model:                 "sonnet",
		InactivityTimeoutMins: 60,
		WorkDir:               workDir,
		PlanningDir:           filepath.Join(workDir, ".planning"),
	}
}

// Executor runs plans using Claude Code
type Executor struct {
	config *Config
	claude *llm.Claude
}

// New creates a new executor
func New(config *Config) *Executor {
	return &Executor{
		config: config,
		claude: llm.NewClaude(config.ClaudeBinary),
	}
}

// RunResult holds the result of a plan execution
type RunResult struct {
	PlanPath    string
	Success     bool
	Duration    time.Duration
	Error       error
	TasksFailed []string
}

// ExecutePlan runs a single plan and returns the result
func (e *Executor) ExecutePlan(ctx context.Context, phase *state.Phase, plan *state.Plan) *RunResult {
	start := time.Now()
	result := &RunResult{
		PlanPath: plan.Path,
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	fmt.Printf("[%s] Executing: %s\n", time.Now().Format("15:04:05"), cyan(plan.Name))

	// Build the execution prompt
	prompt := e.buildExecutionPrompt(plan.Path)

	// Execute with Claude
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "PROJECT.md"),
			filepath.Join(e.config.PlanningDir, "STATE.md"),
		},
		Model: e.config.Model,
		AllowedTools: []string{
			"Read", "Write", "Edit", "Bash", "Glob", "Grep",
			"Task", "TodoWrite", "WebFetch", "WebSearch",
		},
		WorkDir: e.config.WorkDir,
	}

	// Run Claude in interactive mode (passes through stdin/stdout)
	err := e.claude.ExecuteInteractive(ctx, opts)
	if err != nil {
		result.Error = fmt.Errorf("execution failed: %w", err)
		fmt.Printf("[%s] %s Execution failed: %v\n", time.Now().Format("15:04:05"), red("✗"), err)
	} else {
		result.Success = true
		fmt.Printf("[%s] %s Plan complete!\n", time.Now().Format("15:04:05"), green("✓"))
	}

	result.Duration = time.Since(start)
	return result
}

// Loop runs multiple plans until all complete or failure
func (e *Executor) Loop(ctx context.Context, maxIterations int) error {
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	fmt.Println(bold("=== Ralph Autonomous Loop ==="))
	fmt.Println()

	for i := 1; i <= maxIterations; i++ {
		// Reload phases to get current state
		phases, err := state.LoadPhases(e.config.PlanningDir)
		if err != nil {
			return fmt.Errorf("cannot load phases: %w", err)
		}

		// Find next plan
		phase, plan := state.FindNextPlan(phases)
		if plan == nil {
			fmt.Printf("\n%s All plans complete!\n", green("✓"))
			return nil
		}

		total, completed := state.CountPlans(phases)
		fmt.Printf("Iteration %d/%d: %s (%d/%d plans done)\n",
			i, maxIterations, cyan(plan.Name), completed, total)

		// Execute the plan
		result := e.ExecutePlan(ctx, phase, plan)

		if !result.Success {
			fmt.Printf("\n%s FAILED: %s\n", red("✗"), plan.Name)
			if result.Error != nil {
				fmt.Printf("   Error: %v\n", result.Error)
			}
			fmt.Printf("\nStopping loop. %d plans complete, 1 failed.\n", completed)
			fmt.Println("Run 'ralph status' for details.")
			return result.Error
		}

		fmt.Printf("%s Complete (%s)\n\n", green("✓"), result.Duration.Round(time.Second))
	}

	fmt.Printf("\nReached max iterations (%d). Run 'ralph run --loop' to continue.\n", maxIterations)
	return nil
}

func (e *Executor) buildExecutionPrompt(planPath string) string {
	return fmt.Sprintf(`You are executing a plan autonomously. Follow the plan exactly.

## Plan Location
%s

## Execution Protocol

1. Read the PLAN.md file carefully
2. Execute each task in order
3. After each task:
   - Run the <verify> command if present
   - If verification fails, try to fix the issue once
   - If still fails, signal failure and stop
4. Create atomic git commits after each task:
   git commit -m "{type}({phase}-{plan}): {task_name}"
5. After ALL tasks complete:
   - Run all checks in <verification> section
   - Create SUMMARY.md in the phase directory
   - Signal: ###PLAN_COMPLETE###

## Failure Signals
- ###TASK_FAILED:{name}### - A task couldn't be completed
- ###PLAN_FAILED:{check}### - Plan verification failed
- ###BLOCKED:{reason}### - Need human intervention

## Rules
- NO placeholders or stub implementations
- NO skipping verification
- NO continuing after failure
- If uncertain, signal: ###BLOCKED:uncertain###

Begin execution now.`, planPath)
}

// CheckGSDInstalled verifies GSD is installed
func CheckGSDInstalled() error {
	// Check if GSD commands are available by checking for the skill files
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find home directory: %w", err)
	}

	// Check global install
	globalPath := filepath.Join(home, ".claude", "commands", "gsd")
	if _, err := os.Stat(globalPath); err == nil {
		return nil
	}

	// Check local install
	cwd, _ := os.Getwd()
	localPath := filepath.Join(cwd, ".claude", "commands", "gsd")
	if _, err := os.Stat(localPath); err == nil {
		return nil
	}

	return fmt.Errorf(`GSD (Get Shit Done) not installed

Install with:
  npx get-shit-done-cc --global

Or for local install:
  npx get-shit-done-cc --local`)
}

// CheckClaudeInstalled verifies Claude Code CLI is available
func CheckClaudeInstalled() error {
	// Try to find claude in PATH
	if _, err := exec.LookPath("claude"); err == nil {
		return nil
	}

	// Check common locations
	home, _ := os.UserHomeDir()
	commonPaths := []string{
		filepath.Join(home, ".claude", "local", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
	}

	return fmt.Errorf(`Claude Code CLI not found

To fix, add to your ~/.zshrc or ~/.bashrc:
  export PATH="$HOME/.claude/local:$PATH"

Then restart your terminal, or run:
  source ~/.zshrc`)
}

// resolveBinaryPath finds a binary, checking common locations
func resolveBinaryPath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}

	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	home, _ := os.UserHomeDir()
	if strings.HasPrefix(name, "~") {
		return filepath.Join(home, name[1:])
	}

	return name
}
