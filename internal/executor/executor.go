package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/state"
	"github.com/fatih/color"
)

// gsdWorkflowPath returns the path to the GSD execute-phase workflow
func gsdWorkflowPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "get-shit-done", "workflows", "execute-phase.md")
}

// loadGSDWorkflow attempts to load the GSD execute-phase workflow from the user's home directory
func loadGSDWorkflow() (string, error) {
	gsdPath := gsdWorkflowPath()
	if gsdPath == "" {
		return "", fmt.Errorf("HOME not set")
	}

	content, err := os.ReadFile(gsdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("GSD workflow not found at %s", gsdPath)
		}
		return "", err
	}

	return string(content), nil
}

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

	// Create a cancellable context for process termination on signals
	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()

	// Run Claude in streaming mode to capture output and signals
	reader, err := e.claude.Execute(execCtx, opts)
	if err != nil {
		result.Error = fmt.Errorf("execution failed: %w", err)
		fmt.Printf("[%s] %s Execution failed: %v\n", time.Now().Format("15:04:05"), red("✗"), err)
		result.Duration = time.Since(start)
		return result
	}
	defer reader.Close()

	// Parse the stream output, with termination callback for failure/bailout signals
	handler := llm.NewConsoleHandler()
	if err := llm.ParseStream(reader, handler, cancelExec); err != nil {
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
		// Verify SUMMARY.md was created before marking success
		summaryPath := strings.Replace(plan.Path, "-PLAN.md", "-SUMMARY.md", 1)
		if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
			result.Error = fmt.Errorf("plan signaled complete but SUMMARY.md not created: %s", summaryPath)
			result.FailureType = FailureSoft
			fmt.Printf("[%s] %s Plan signaled complete but SUMMARY.md missing\n", time.Now().Format("15:04:05"), yellow("⚠"))
		} else {
			// Success - explicit completion signal with SUMMARY.md verified
			result.Success = true
			result.FailureType = FailureNone
			fmt.Printf("[%s] %s Plan complete!\n", time.Now().Format("15:04:05"), green("✓"))

			// Update STATE.md and ROADMAP.md with new progress
			phases, _ := state.LoadPhases(e.config.PlanningDir)
			if err := state.UpdateStateFile(e.config.PlanningDir, phases); err != nil {
				fmt.Printf("[%s] %s Failed to update STATE.md: %v\n",
					time.Now().Format("15:04:05"), yellow("⚠"), err)
			}
			if err := state.UpdateRoadmap(e.config.PlanningDir, phases); err != nil {
				fmt.Printf("[%s] %s Failed to update ROADMAP.md: %v\n",
					time.Now().Format("15:04:05"), yellow("⚠"), err)
			}

			// Commit and push all repos
			planId := fmt.Sprintf("%02d-%02d", phase.Number, plan.Number)
			e.CommitAndPushRepos(planId)
		}
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

	var lastPhaseNumber int = -1
	var lastPlanPath string
	var lastProgressContent string

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

		// Check for stuck loop - same plan found twice with NO progress made
		// This allows bailout recovery (same plan, but Progress section updated)
		if plan.Path == lastPlanPath {
			currentProgress := extractProgressSection(plan.Path)
			if currentProgress == lastProgressContent {
				// Same progress = truly stuck (no work was done)
				return fmt.Errorf("stuck on plan %s - Progress section unchanged after execution", plan.Name)
			}
			// Progress section was updated = bailout recovery, allow continuation
			fmt.Printf("[%s] %s Resuming %s (Progress updated, continuing)\n",
				time.Now().Format("15:04:05"), cyan("↻"), plan.Name)
		}
		lastPlanPath = plan.Path
		lastProgressContent = extractProgressSection(plan.Path)

		// Check if we're starting a new phase - if so, scan for decision checkpoints
		if phase.Number != lastPhaseNumber {
			lastPhaseNumber = phase.Number
			created, err := e.MaybeCreateDecisionsPlan(phase)
			if err != nil {
				fmt.Printf("[%s] %s Failed to create decisions plan: %v\n",
					time.Now().Format("15:04:05"), yellow("⚠"), err)
			} else if created {
				// Decisions plan was created - reload phases and find next plan again
				phases, err = state.LoadPhases(e.config.PlanningDir)
				if err != nil {
					return fmt.Errorf("cannot reload phases after decisions plan: %w", err)
				}
				phase, plan = state.FindNextPlan(phases)
				if plan == nil {
					fmt.Printf("\n%s All plans complete!\n", green("✓"))
					return nil
				}
			}
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
		} else if analysisResult.ObservationsFound > 0 {
			fmt.Printf("   Analyzed %d discoveries\n", analysisResult.ObservationsFound)
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
			fmt.Println()
		} else {
			// Only print checkmark on actual success
			fmt.Printf("%s Complete (%s)\n", green("✓"), result.Duration.Round(time.Second))
			fmt.Println()
		}
	}

	fmt.Printf("\nReached max iterations (%d). Run 'ralph run --loop' to continue.\n", maxIterations)
	return nil
}

func (e *Executor) buildExecutionPrompt(planPath string) string {
	// Try to load GSD workflow as base
	gsdBase, err := loadGSDWorkflow()
	if err != nil {
		// Log warning and use fallback
		yellow := color.New(color.FgYellow).SprintFunc()
		fmt.Fprintf(os.Stderr, "[%s] %s GSD workflow not available: %v (using fallback)\n",
			time.Now().Format("15:04:05"), yellow("⚠"), err)
		return e.buildFallbackExecutionPrompt(planPath)
	}

	// Build Ralph-specific augmentations
	ralphAugmentations := buildRalphAugmentations(planPath)

	return gsdBase + "\n\n" + ralphAugmentations
}

// buildRalphAugmentations returns Ralph-specific additions to append to GSD workflow
func buildRalphAugmentations(planPath string) string {
	return fmt.Sprintf(`---

## Ralph Augmentations

The following are Ralph-specific extensions to the GSD execution workflow.

### Plan Location
%s

### Observation Types (Ralph-specific)

Ralph's analysis agent parses observations to improve subsequent plans. Types:
- **bug**: Bug found that needs fixing
- **stub**: Stub/placeholder code that needs implementation
- **api-issue**: External API problem or inconsistency
- **insight**: Useful information for future plans
- **blocker**: Something blocking progress
- **technical-debt**: Code that works but needs improvement
- **assumption**: Decision made without full information
- **scope-creep**: Work discovered that wasn't in the plan
- **dependency**: Unexpected dependency between tasks/plans
- **questionable**: Suspicious code or pattern worth reviewing
- **already-complete**: Work was already done before execution
- **checkpoint-automated**: Checkpoint verification that was automated
- **tooling-friction**: Tool/environment issue that slowed progress
- **test-failed**: Test(s) failed during execution - enumerate test names
- **test-infrastructure**: Test environment issue (simulator, timeout, xcodebuild syntax)

### Observation Format (CRITICAL - Analyzer Cannot Parse Prose)

**IMPORTANT**: Prose observations like "## Discovery: ..." or "**Finding:** ..." CANNOT be parsed.
You MUST use this exact XML format:

` + "```" + `xml
<observation type="TYPE" severity="SEVERITY">
  <title>Short descriptive title</title>
  <detail>What you found and why it matters</detail>
  <file>path/to/relevant/file</file>
  <action>ACTION</action>
</observation>
` + "```" + `

**Severities**: critical, high, medium, low, info
**Actions**: needs-fix, needs-implementation, needs-plan, needs-investigation, needs-documentation, needs-human-verify, none

### What to Observe (LOW BAR - Record Everything)

Record routine findings. Examples:
- "3 tests in X are stubs" → type="stub", action="needs-implementation"
- "File Y has no tests" → type="insight", action="needs-plan"
- "Function Z is deprecated but used in 5 places" → type="technical-debt", action="needs-fix"
- "This took 30 min because docs were wrong" → type="tooling-friction", action="needs-documentation"
- "Tests already exist for X" → type="already-complete", action="none"
- "Found TODO comment in code" → type="stub", action="needs-implementation"
- "API returns different format than docs say" → type="api-issue", action="needs-investigation"

**The analysis agent needs DATA to work with. Under-reporting = no analysis happens.**

### Documenting Test Failures (CRITICAL)

Test failures require STRUCTURED observations, not prose notes. The analysis agent:
- Can detect patterns across plans (e.g., "xcodebuild issues in 4/5 plans")
- Can recommend infrastructure fixes when issues repeat
- CANNOT parse prose like "tests failed, see output"

**When tests fail:**
1. Use type="test-failed" (NOT generic "blocker")
2. List EACH failed test by name
3. Include error messages or expected vs actual
4. For tooling issues (xcodebuild syntax), use type="test-infrastructure"

**Example - Test Failures:**
` + "```" + `xml
<observation type="test-failed" severity="high">
  <title>3 SpatialAudioService tests failing</title>
  <detail>
    Failed tests:
    - testPlaySpatialAudio_atPosition: Expected position (1,2,3), got (0,0,0)
    - testStopAllSpatialAudio: Source still playing after stop
    - testPauseSpatialAudio: Playback not paused
    Root cause: uninitialized position variable in SpatialAudioService.play()
  </detail>
  <file>ar/AR/Unit Tests iOS/AudioSpatialTests.swift</file>
  <action>needs-fix</action>
</observation>
` + "```" + `

**Example - Test Infrastructure:**
` + "```" + `xml
<observation type="test-infrastructure" severity="medium">
  <title>xcodebuild -only-testing syntax unclear</title>
  <detail>
    Spent 30+ minutes on xcodebuild test filtering. Attempted syntaxes:
    - -only-testing:TestTarget/TestClass (Unknown build action error)
    - -only-testing "TestTarget/TestClass" (same error)
    Documentation unclear. This blocked test verification for SpatialAudioService.
  </detail>
  <file>ar/AR/AR.xcodeproj</file>
  <action>needs-documentation</action>
</observation>
` + "```" + `

### Recording Observations (Use Subagents to Save Context)

Recording observations inline burns your main context. Use Task tool to delegate:

` + "```" + `
Task(subagent_type="general-purpose", prompt="
  Add this observation to PLAN.md in the Observations section:
  <observation type=\"stub\" severity=\"medium\">
    <title>3 backend tests are stubs</title>
    <detail>image.test.ts and video.test.ts have stub tests</detail>
    <file>mix-backend/functions/src/__tests__/endpoints/</file>
    <action>needs-implementation</action>
  </observation>
")
` + "```" + `

Record observations AS YOU GO - don't batch them at the end.

### Post-Execution Analysis

After you complete (or fail), Ralph spawns an analysis agent to:
1. Parse all observations from this execution
2. Review subsequent plans for impact
3. Potentially restructure plans (reorder, create new, skip completed)

To maximize effectiveness:
- Record observations AS YOU FIND THEM
- Be specific about dependencies discovered
- Flag assumptions that might affect future plans

### Pre-Existing Work Handling (IMPORTANT)

When you find that work in a task is ALREADY COMPLETE (files exist, code already implemented):

1. **Record an observation:**
   ` + "```" + `xml
   <observation type="already-complete" severity="info">
     <title>Task N already implemented</title>
     <detail>The [what] already exists at [path]. Likely done in previous session.</detail>
     <file>path/to/existing/file</file>
     <action>none</action>
   </observation>
   ` + "```" + `

2. **Update Progress section:** Mark task as ` + "`[ALREADY_COMPLETE]`" + `

3. **Verify existing work meets requirements** - if partial, complete it

4. **Continue normally:** Create SUMMARY.md, signal ###PLAN_COMPLETE###

**DO NOT** get stuck investigating history. Document what exists and move forward.

### Background Task Verification (MANDATORY)

BEFORE signaling ###PLAN_COMPLETE###, you MUST verify all background tasks have finished:

1. **Check for running background tasks:**
   - If you started tests/builds with ` + "`run_in_background: true`" + `, you MUST wait for completion
   - Read the output file (from TaskOutput) to verify tests completed AND passed
   - Use Bash: ` + "`ps aux | grep -E \"(xcodebuild|npm test|pytest|go test)\" | grep -v grep`" + ` to check for running processes

2. **You CANNOT signal ###PLAN_COMPLETE### if:**
   - Background tests are still running (check process list)
   - You haven't read and verified test output shows passing
   - Build processes are still executing
   - SUMMARY.md has not been created

3. **Verification sequence:**
   a. Wait for all background tasks to finish (use TaskOutput with block=true)
   b. Verify test results show PASS (not just "started" or "running")
   c. Create SUMMARY.md with execution details
   d. Only then signal ###PLAN_COMPLETE###

Ralph will verify SUMMARY.md exists before accepting the completion signal.

### Ralph Signals

Ralph monitors for these signals in addition to GSD signals:
- ###PLAN_COMPLETE### - All tasks done, verified, builds and tests pass
- ###TASK_FAILED:{name}### - A task couldn't be completed
- ###PLAN_FAILED:{check}### - Plan verification failed
- ###BUILD_FAILED:{project}### - Build failed (e.g., ios, backend)
- ###TEST_FAILED:{project}:{count}### - Tests failed that weren't failing before
- ###BLOCKED:{reason}### - Need human intervention
- ###BAILOUT:{reason}### - Preserving context, update Progress first

### Context Management (Ralph Safety Net)

Ralph monitors your token usage and will terminate at 120K tokens as a safety net.

**Self-monitoring heuristics:**
- Count your tool calls: if > 50 tool calls without task completion, you're burning context
- Watch for repeated errors: 3+ retries of same fix = stuck, bail out
- File reading volume: if you've read > 20 files without progress, context is bloated

**Use subagents for writing to save context:**
- Use Task tool (subagent_type="general-purpose") for recording observations and progress updates
- Prompt: "Update PLAN.md with observations: [list what you found]. Update Progress section: [current state]"
- This offloads file editing work to a fresh subagent context, preserving your main context for execution

**At ~100K tokens, proactively bail out:**
1. Update the PLAN.md Progress section with current state
2. Update the ## Observations section with any findings
3. Document what worked, what failed, and next steps
4. Signal: ###BAILOUT:context_preservation###

Begin execution now.`, planPath)
}

// buildFallbackExecutionPrompt returns the standalone execution prompt when GSD is unavailable
func (e *Executor) buildFallbackExecutionPrompt(planPath string) string {
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

## Observation Recording (MANDATORY - LOW BAR)

**CRITICAL: The analyzer CANNOT parse prose. You MUST use XML format.**

Record observations LIBERALLY. Low-bar examples:
- "3 tests are stubs" → type="stub"
- "File X has no tests" → type="insight"
- "Function Y deprecated but still used" → type="technical-debt"
- "Took 30 min because docs wrong" → type="tooling-friction"

Add a ## Observations section to PLAN.md with XML entries:

`+"```"+`xml
<observation type="TYPE" severity="SEVERITY">
  <title>Brief title</title>
  <detail>What you found and why it matters</detail>
  <file>path/to/relevant/file.ts</file>
  <action>ACTION</action>
</observation>
`+"```"+`

**Types:** bug, stub, api-issue, insight, blocker, technical-debt, tooling-friction, env-discovery, assumption, scope-creep, dependency, questionable, already-complete, checkpoint-automated

**Severity:** critical, high, medium, low, info
**Actions:** needs-fix, needs-implementation, needs-plan, needs-investigation, needs-documentation, needs-human-verify, none

Example observations:
<observation type="tooling-friction" severity="info">
  <title>Xcode test target naming</title>
  <detail>Test target is "Unit Tests iOS", not "Tests iOS". Found via xcodebuild -list</detail>
  <file>ar/AR/AR.xcodeproj</file>
  <action>needs-documentation</action>
</observation>

<observation type="scope-creep" severity="high">
  <title>Need to update 3 additional files</title>
  <detail>The auth change requires updating UserService, ProfileView, and SettingsView which weren't in the plan.</detail>
  <action>needs-plan</action>
</observation>

Record observations AS YOU GO - don't batch at end. Under-reporting = no analysis happens.

## Pre-Existing Work Handling (IMPORTANT)

When you find work is ALREADY COMPLETE:

1. **Record an observation:**
   ` + "```" + `xml
   <observation type="already-complete" severity="info">
     <title>Task N already implemented</title>
     <detail>The [what] already exists at [path]. Likely done in previous session.</detail>
     <file>path/to/existing/file</file>
     <action>none</action>
   </observation>
   ` + "```" + `

2. **Update Progress section:** Mark task as ` + "`[ALREADY_COMPLETE]`" + `

3. **Verify existing work meets requirements** - if partial, complete it

4. **Continue normally:** Create SUMMARY.md, signal ###PLAN_COMPLETE###

**DO NOT** get stuck investigating history. Document what exists and move forward.

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

## Background Task Verification (MANDATORY)

If you started ANY tasks with ` + "`" + `run_in_background: true` + "`" + `, you MUST verify completion:

1. **Wait for background tasks to finish:**
   - Use TaskOutput tool with block=true to wait for completion
   - Read the output file to verify results

2. **Check no processes are still running:**
   - Use Bash: ` + "`" + `ps aux | grep -E "(xcodebuild|npm test|pytest|go test)" | grep -v grep` + "`" + `
   - Empty output = no running processes

3. **Verify test results show PASS:**
   - Read the test output file
   - Look for "PASS" or "succeeded" (not just "started" or "running")
   - Count actual test results, not just the command starting

4. **Create SUMMARY.md BEFORE signaling:**
   - Ralph will verify SUMMARY.md exists before accepting the completion signal
   - If SUMMARY.md is missing, your ###PLAN_COMPLETE### will be rejected

**Verification sequence:**
a. Wait for all background tasks (TaskOutput with block=true)
b. Verify test output shows actual PASS results
c. Create SUMMARY.md with execution details
d. Only then signal ###PLAN_COMPLETE###

## Context Management (CRITICAL)

You have ~200K tokens of context. Quality degrades significantly after ~100K tokens.
Ralph is monitoring your token usage and will terminate at 120K tokens as a safety net.

**Self-monitoring heuristics:**
- Count your tool calls: if > 50 tool calls without task completion, you're burning context
- Watch for repeated errors: 3+ retries of same fix = stuck, bail out
- File reading volume: if you've read > 20 files without progress, context is bloated

**Use subagents for writing to save context:**
- Use Task tool (subagent_type="general-purpose") for recording observations and progress updates
- Prompt: "Update PLAN.md with observations: [list what you found]. Update Progress section: [current state]"
- This offloads file editing work to a fresh subagent context, preserving your main context for execution

**At ~100K tokens, proactively bail out:**
1. Update the PLAN.md Progress section with current state
2. Update the ## Observations section with any findings
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
- ALWAYS record observations in XML format as you find them
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

// extractProgressSection returns the content of the ## Progress section from a plan file
// Used to detect if progress was made between executions (for stuck loop detection)
func extractProgressSection(planPath string) string {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return ""
	}

	// Extract everything after ## Progress until the next section or EOF
	re := regexp.MustCompile(`(?s)## Progress\n(.*?)(?:## |\z)`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
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

// DecisionCheckpoint represents a checkpoint:decision extracted from a plan
type DecisionCheckpoint struct {
	PlanNumber  int
	PlanName    string
	PlanPath    string
	TaskContent string
	Context     string
}

// MaybeCreateDecisionsPlan scans a phase for checkpoint:decision tasks and creates a bundled decisions plan
func (e *Executor) MaybeCreateDecisionsPlan(phase *state.Phase) (bool, error) {
	cyan := color.New(color.FgCyan).SprintFunc()

	// Check if decisions plan already exists
	decisionsPath := filepath.Join(phase.Path, fmt.Sprintf("%02d-00-decisions-PLAN.md", phase.Number))
	if _, err := os.Stat(decisionsPath); err == nil {
		// Decisions plan already exists
		return false, nil
	}

	// Scan all plans in this phase for checkpoint:decision tasks
	var decisions []DecisionCheckpoint
	decisionPattern := regexp.MustCompile(`(?s)<task\s+type="checkpoint:decision"[^>]*>(.*?)</task>`)

	for _, plan := range phase.Plans {
		content, err := os.ReadFile(plan.Path)
		if err != nil {
			continue
		}

		matches := decisionPattern.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				decisions = append(decisions, DecisionCheckpoint{
					PlanNumber:  plan.Number,
					PlanName:    plan.Name,
					PlanPath:    plan.Path,
					TaskContent: strings.TrimSpace(match[1]),
				})
			}
		}
	}

	if len(decisions) == 0 {
		// No decisions found in this phase
		return false, nil
	}

	fmt.Printf("[%s] %s Found %d decision checkpoints, creating decisions plan...\n",
		time.Now().Format("15:04:05"), cyan("Decisions:"), len(decisions))

	// Create the decisions plan
	err := e.createDecisionsPlan(phase, decisions, decisionsPath)
	if err != nil {
		return false, err
	}

	fmt.Printf("[%s] %s Created %s\n",
		time.Now().Format("15:04:05"), cyan("Decisions:"), filepath.Base(decisionsPath))

	return true, nil
}

// createDecisionsPlan generates the bundled decisions plan file
func (e *Executor) createDecisionsPlan(phase *state.Phase, decisions []DecisionCheckpoint, outPath string) error {
	var content strings.Builder

	// Write frontmatter
	content.WriteString(fmt.Sprintf(`---
phase: %d
plan: 0
type: decisions
status: pending
---

# Phase %d Decisions: Upfront Choices

## Objective

Make all architectural and approach decisions before executing phase plans.
These decisions will be referenced by subsequent plans via STATE.md.

## Decisions Required

`, phase.Number, phase.Number))

	// Write each decision
	for i, d := range decisions {
		affectedPlans := fmt.Sprintf("%02d-%02d", phase.Number, d.PlanNumber)
		content.WriteString(fmt.Sprintf(`### Decision %d: From Plan %s

**Original plan:** %s
**Affects:** Plan %s and potentially subsequent plans

%s

<task type="checkpoint:decision">
%s
</task>

`, i+1, d.PlanName, d.PlanPath, affectedPlans, d.Context, d.TaskContent))
	}

	// Write recording instructions
	content.WriteString(`## Recording Decisions

After each decision is made:
1. Record the decision in STATE.md Decisions table
2. Include the rationale
3. Note which plans are affected

Subsequent plans will read STATE.md to access these decisions.

## Verification

<verification>
- [ ] All decisions have been made
- [ ] Each decision is recorded in STATE.md
- [ ] Decision rationales are documented
</verification>

## Success Criteria

- All architectural and approach decisions are finalized
- STATE.md Decisions table is populated with all choices
- Team is aligned on the approach before execution begins
`)

	return os.WriteFile(outPath, []byte(content.String()), 0644)
}

// CommitAndPushRepos commits and pushes changes in all workspace repos
func (e *Executor) CommitAndPushRepos(planId string) error {
	// Find all git repos in workspace (submodules or sibling repos)
	repos := []string{
		e.config.WorkDir, // Main workspace (e.g., mix/)
	}

	// Check for common submodule/sibling patterns
	possibleRepos := []string{"ar", "mix-backend", "mix-dashboard", "mix-web", "plans", "ralph"}
	for _, name := range possibleRepos {
		repoPath := filepath.Join(e.config.WorkDir, name)
		gitPath := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			repos = append(repos, repoPath)
		}
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	for _, repo := range repos {
		// Check if there are changes to commit
		statusCmd := exec.Command("git", "-C", repo, "status", "--porcelain")
		output, _ := statusCmd.Output()
		if len(output) == 0 {
			continue // No changes in this repo
		}

		repoName := filepath.Base(repo)
		fmt.Printf("[%s] %s Committing changes in %s\n",
			time.Now().Format("15:04:05"), cyan("Git:"), repoName)

		// Stage all changes
		addCmd := exec.Command("git", "-C", repo, "add", "-A")
		if err := addCmd.Run(); err != nil {
			fmt.Printf("[%s] %s Failed to stage in %s: %v\n",
				time.Now().Format("15:04:05"), yellow("⚠"), repoName, err)
			continue
		}

		// Commit with plan reference
		commitMsg := fmt.Sprintf("chore(%s): auto-commit after plan completion", planId)
		commitCmd := exec.Command("git", "-C", repo, "commit", "-m", commitMsg)
		if err := commitCmd.Run(); err != nil {
			fmt.Printf("[%s] %s Failed to commit in %s: %v\n",
				time.Now().Format("15:04:05"), yellow("⚠"), repoName, err)
			continue
		}

		// Push to current branch
		pushCmd := exec.Command("git", "-C", repo, "push")
		if err := pushCmd.Run(); err != nil {
			fmt.Printf("[%s] %s Failed to push %s: %v\n",
				time.Now().Format("15:04:05"), yellow("⚠"), repoName, err)
			continue
		}

		fmt.Printf("[%s] %s Pushed %s\n",
			time.Now().Format("15:04:05"), green("✓"), repoName)
	}

	return nil
}
