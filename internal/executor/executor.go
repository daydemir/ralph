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

// FailureType indicates the severity of a failure
type FailureType int

const (
	FailureNone FailureType = iota // No failure
	FailureHard                     // Task/verification failed - stop loop
	FailureSoft                     // Signal missing or bailout - continue loop
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
	FailureType FailureType
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
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("[%s] Executing: %s\n", time.Now().Format("15:04:05"), cyan(plan.Name))

	// Build the execution prompt
	prompt := e.buildExecutionPrompt(plan.Path)

	// Execute with Claude using streaming mode
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

	// Run Claude in streaming mode to capture output and signals
	reader, err := e.claude.Execute(ctx, opts)
	if err != nil {
		result.Error = fmt.Errorf("execution failed: %w", err)
		fmt.Printf("[%s] %s Execution failed: %v\n", time.Now().Format("15:04:05"), red("✗"), err)
		result.Duration = time.Since(start)
		return result
	}
	defer reader.Close()

	// Parse the stream output
	handler := llm.NewConsoleHandler()
	if err := llm.ParseStream(reader, handler); err != nil {
		result.Error = fmt.Errorf("stream parsing failed: %w", err)
		fmt.Printf("[%s] %s Stream parsing failed: %v\n", time.Now().Format("15:04:05"), red("✗"), err)
		result.Duration = time.Since(start)
		return result
	}

	// Check for proper completion
	tokenStats := handler.GetTokenStats()
	fmt.Printf("[%s] Tokens used: %d (input: %d, output: %d)\n",
		time.Now().Format("15:04:05"), tokenStats.TotalTokens, tokenStats.InputTokens, tokenStats.OutputTokens)

	if handler.HasFailed() {
		// Hard failure - task/plan/build/test failed
		failure := handler.GetFailure()
		result.Error = fmt.Errorf("%s: %s", failure.Type, failure.Detail)
		result.TasksFailed = []string{failure.Detail}
		result.FailureType = FailureHard
		fmt.Printf("[%s] %s %s: %s\n", time.Now().Format("15:04:05"), red("✗"), failure.Type, failure.Detail)
	} else if handler.IsPlanComplete() {
		// Success - explicit completion signal
		result.Success = true
		result.FailureType = FailureNone
		fmt.Printf("[%s] %s Plan complete!\n", time.Now().Format("15:04:05"), green("✓"))
	} else if handler.IsBailout() {
		// Bailout signal - Claude preserved context, check if Progress was updated
		bailout := handler.GetBailout()
		progressUpdated := e.verifyProgressUpdated(plan.Path)
		if progressUpdated {
			// Soft success - work preserved, can resume
			result.Success = false // Not fully complete, but progress saved
			result.FailureType = FailureSoft
			fmt.Printf("[%s] %s Bailout with progress preserved: %s\n", time.Now().Format("15:04:05"), cyan("↻"), bailout.Detail)
			fmt.Println("   Progress section updated. Run 'ralph run' to continue.")
		} else {
			// Bailout without progress update - warn user
			result.Error = fmt.Errorf("bailout without progress update: %s", bailout.Detail)
			result.FailureType = FailureSoft
			fmt.Printf("[%s] %s Bailout WITHOUT progress update: %s\n", time.Now().Format("15:04:05"), yellow("⚠"), bailout.Detail)
			fmt.Println("   Warning: Progress section may not be updated. Check PLAN.md manually.")
		}
	} else if handler.ShouldBailOut() {
		// Token limit reached - Ralph's safety net triggered
		result.Error = fmt.Errorf("token limit reached: %d tokens", tokenStats.TotalTokens)
		result.FailureType = FailureSoft
		fmt.Printf("[%s] %s Token limit bailout at %d tokens\n", time.Now().Format("15:04:05"), yellow("⚠"), tokenStats.TotalTokens)
	} else {
		// No signal at all - Claude exited without any completion/failure signal
		result.FailureType = FailureSoft
		fmt.Printf("[%s] %s Claude exited without completion signal\n", time.Now().Format("15:04:05"), yellow("⚠"))
		fmt.Println("   Work may be complete. Continuing to next plan...")
	}

	result.Duration = time.Since(start)
	return result
}

// Loop runs multiple plans until all complete or failure
func (e *Executor) Loop(ctx context.Context, maxIterations int) error {
	return e.LoopWithAnalysis(ctx, maxIterations, false)
}

// LoopWithAnalysis runs multiple plans with optional post-analysis
func (e *Executor) LoopWithAnalysis(ctx context.Context, maxIterations int, skipAnalysis bool) error {
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
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

		// Run post-analysis ALWAYS - even on hard failures - to diagnose issues and update plans
		analysisResult := e.RunPostAnalysis(ctx, phase, plan, skipAnalysis)
		if analysisResult.Error != nil {
			fmt.Printf("   Warning: post-analysis failed: %v\n", analysisResult.Error)
		} else if analysisResult.DiscoveriesFound > 0 {
			fmt.Printf("   Analyzed %d discoveries\n", analysisResult.DiscoveriesFound)
		}

		if !result.Success {
			if result.FailureType == FailureHard {
				// Hard failure - stop the loop (analysis already ran above)
				fmt.Printf("\n%s FAILED: %s\n", red("✗"), plan.Name)
				if result.Error != nil {
					fmt.Printf("   Error: %v\n", result.Error)
				}
				fmt.Printf("\nStopping loop. %d plans complete, 1 failed.\n", completed)
				fmt.Println("Run 'ralph status' for details.")
				return result.Error
			}
			// Soft failure - warn but continue to next plan
			fmt.Printf("\n%s %s (soft failure, continuing to next plan)\n", yellow("⚠"), plan.Name)
			if result.Error != nil {
				fmt.Printf("   Warning: %v\n", result.Error)
			}
		}

		fmt.Printf("%s Complete (%s)\n", green("✓"), result.Duration.Round(time.Second))
		fmt.Println()
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
3. **CRITICAL: After each task, update the PLAN.md file's Progress section**
4. After each task:
   - Run the <verify> command if present
   - If verification fails, try to fix the issue once
   - If still fails, signal failure and stop
5. Create atomic git commits after each task:
   git commit -m "{type}({phase}-{plan}): {task_name}"
6. After ALL tasks complete:
   - Run all checks in <verification> section
   - Run build and test verification (see below)
   - Create SUMMARY.md in the phase directory
   - Signal: ###PLAN_COMPLETE###

## Progress Tracking (MANDATORY)

After completing each task, add/update a ## Progress section at the end of the PLAN.md file:

`+"```"+`markdown
## Progress
- Task 1: [COMPLETE] - What was done, verification passed
- Task 2: [IN_PROGRESS] - Current state, any blockers
- Task 3: [PENDING]
`+"```"+`

This ensures the next run can continue where you left off if context runs low.

## Discovery Recording (MANDATORY)

During execution, record ANY findings that don't fit the plan:
- Tests that are stubs but marked as passing
- APIs behaving differently than documented
- Bugs found in existing code
- Code that already exists (would have been duplicated)
- Unexpected dependencies or side effects
- Assumptions you made without full information
- Work discovered that wasn't anticipated in the plan (scope creep)
- Suspicious code or patterns worth reviewing
- Decisions that future plans might need to know about

Add a ## Discoveries section to the PLAN.md file with XML-structured entries:

`+"```"+`markdown
## Discoveries

<discovery type="TYPE" severity="SEVERITY">
  <title>Brief title</title>
  <detail>What you found and why it matters</detail>
  <file>path/to/relevant/file.ts</file>
  <action>ACTION</action>
</discovery>
`+"```"+`

**Types:**
- bug: Existing bug found in codebase
- stub: Tests or code that are placeholders
- api-issue: External API behaving unexpectedly
- insight: Useful pattern or approach discovered
- blocker: Something preventing progress
- technical-debt: Code quality issue found
- tooling-friction: Build/test quirks learned through trial-and-error
- env-discovery: Environment setup learned
- assumption: Decision made without full information (IMPORTANT for analysis)
- scope-creep: Work discovered that wasn't in the plan (IMPORTANT for analysis)
- dependency: Unexpected dependency between tasks/plans (IMPORTANT for analysis)
- questionable: Suspicious code or pattern worth reviewing (IMPORTANT for analysis)

**Severity:** critical, high, medium, low, info
**Actions:** needs-fix, needs-implementation, needs-plan, needs-investigation, needs-documentation, none

**Important for analysis agent:** The types marked "IMPORTANT" help the analysis agent decide if subsequent plans need updating, reordering, or if new plans should be created.

**Tooling friction examples** (things discovered through trial-and-error):
- Correct command syntax (e.g., "Test target is 'Unit Tests iOS' not 'Tests iOS'")
- File locations found after searching
- Build/test quirks (e.g., "Must use -destination 'generic/platform=iOS Simulator'")

Example discoveries:
<discovery type="tooling-friction" severity="info">
  <title>Xcode test target naming</title>
  <detail>Test target is "Unit Tests iOS", not "Tests iOS". Found via xcodebuild -list</detail>
  <file>ar/AR/AR.xcodeproj</file>
  <action>needs-documentation</action>
</discovery>

<discovery type="assumption" severity="medium">
  <title>Assumed auth endpoint returns user ID</title>
  <detail>Plan says "get user from auth" but doesn't specify format. Assumed response includes userId field.</detail>
  <file>src/services/auth.ts</file>
  <action>needs-investigation</action>
</discovery>

<discovery type="scope-creep" severity="high">
  <title>Need to update 3 additional files</title>
  <detail>The auth change requires updating UserService, ProfileView, and SettingsView which weren't in the plan.</detail>
  <action>needs-plan</action>
</discovery>

Record discoveries AS YOU FIND THEM, not at the end. This ensures learnings survive context limits and the analysis agent can act on them.

## Build & Test Verification (MANDATORY)

Before signaling ###PLAN_COMPLETE###, you MUST:

1. Run ALL verification checks in the <verification> section
2. Run project build commands:
   - Look for build commands in CLAUDE.md, .ralph/config.yaml, or package.json/Makefile
   - Common: npm run build, xcodebuild, go build, etc.
3. Run project test suite:
   - Look for test commands in CLAUDE.md, .ralph/config.yaml, or package.json
   - Common: npm test, go test, xcodebuild test, etc.
4. If ANY build or test fails that wasn't already failing before your changes:
   - Fix the issue
   - Re-run verification
   - Only then signal completion

You CANNOT signal ###PLAN_COMPLETE### if:
- Build fails
- New test failures introduced
- Verification checks incomplete

If builds/tests fail and you cannot fix them, signal ###BUILD_FAILED:{project}### or ###TEST_FAILED:{project}:{count}###

## Context Management (CRITICAL)

You have ~200K tokens of context. Quality degrades significantly after ~100K tokens.
Ralph is monitoring your token usage and will terminate at 120K tokens as a safety net.

**Self-monitoring heuristics:**
- Count your tool calls: if > 50 tool calls without task completion, you're burning context
- Watch for repeated errors: 3+ retries of same fix = stuck, bail out
- File reading volume: if you've read > 20 files without progress, context is bloated

**At ~100K tokens, proactively bail out:**
1. Update the PLAN.md Progress section with current state
2. Update the ## Discoveries section with any findings
3. Document what worked, what failed, and next steps
4. Signal: ###BAILOUT:context_preservation###

## Signals
- ###PLAN_COMPLETE### - All tasks done, verified, builds and tests pass
- ###TASK_FAILED:{name}### - A task couldn't be completed
- ###PLAN_FAILED:{check}### - Plan verification failed
- ###BUILD_FAILED:{project}### - Build failed (e.g., ios, backend)
- ###TEST_FAILED:{project}:{count}### - Tests failed that weren't failing before
- ###BLOCKED:{reason}### - Need human intervention
- ###BAILOUT:{reason}### - Preserving context, update Progress first

## Rules
- NO placeholders or stub implementations
- NO skipping verification
- NO continuing after failure
- ALWAYS update Progress section after each task
- ALWAYS record discoveries as you find them
- If uncertain, signal: ###BLOCKED:uncertain###
- If burning context without progress, signal: ###BAILOUT:context_preservation###

Begin execution now.`, planPath)
}

// verifyProgressUpdated checks if the PLAN.md file has a ## Progress section
func (e *Executor) verifyProgressUpdated(planPath string) bool {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return false
	}

	// Check for ## Progress section in the file
	return strings.Contains(string(content), "## Progress")
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
