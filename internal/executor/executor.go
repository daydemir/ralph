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

	"github.com/daydemir/ralph/internal/display"
	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/prompts"
	"github.com/daydemir/ralph/internal/state"
	"github.com/daydemir/ralph/internal/types"
)

// loadExecutorPrompt loads the executor agent prompt from internal templates
func loadExecutorPrompt() (string, error) {
	return prompts.GetAgent("executor")
}

// FailureType indicates the severity of a failure
type FailureType int

const (
	FailureNone FailureType = iota // No failure
	FailureHard                    // Task/verification failed - stop loop
	FailureSoft                    // Signal missing or bailout - continue loop
)

// Config holds executor configuration
type Config struct {
	ClaudeBinary          string
	Model                 string
	InactivityTimeoutMins int
	WorkDir               string
	PlanningDir           string
	MaxRetries            int
}

// PlanRetryState tracks retry state for progressive guidance
type PlanRetryState struct {
	Attempts     int
	LastProgress string
	LastOutput   string
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
	config  *Config
	claude  *llm.Claude
	display *display.Display
}

// New creates a new executor
func New(config *Config) *Executor {
	return &Executor{
		config:  config,
		claude:  llm.NewClaude(config.ClaudeBinary),
		display: display.New(),
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
	LastOutput  string
}

// ExecutePlan runs a single plan and returns the result
func (e *Executor) ExecutePlan(ctx context.Context, phase *state.Phase, plan *state.Plan) *RunResult {
	start := time.Now()
	result := &RunResult{
		PlanPath: plan.Path,
	}

	// Check if this is a manual plan - run interactively with gsd:execute-plan
	if plan.IsManual() {
		return e.ExecuteManualPlanInteractive(ctx, phase, plan, start)
	}

	// Validate task types in the plan before execution
	planContent, err := os.ReadFile(plan.Path)
	if err != nil {
		result.Error = fmt.Errorf("failed to read plan: %w", err)
		result.FailureType = FailureHard
		return result
	}
	if err := ValidatePlanTasks(string(planContent)); err != nil {
		result.Error = fmt.Errorf("plan validation failed: %w", err)
		result.FailureType = FailureHard
		e.display.Error(fmt.Sprintf("Invalid task type in plan: %v", err))
		return result
	}

	// Show execution start in a Ralph box
	e.display.RalphBox("RALPH",
		fmt.Sprintf("Executing: %s", plan.Name),
		fmt.Sprintf("Phase %d, Plan %s", phase.Number, plan.Number))

	// Show Claude execution start
	e.display.ClaudeStart()

	// Build the execution prompt
	prompt := e.buildExecutionPrompt(plan.Path)

	// Execute with Claude using streaming mode
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "project.json"),
			filepath.Join(e.config.PlanningDir, "state.json"),
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
		// Execution failed to start - give analyzer context to decide recovery
		execContext := ExecutionContext{
			Error:             err,
			CapturedLogs:      []string{}, // No output captured if execution didn't start
			LastToolCall:      "",
			ClaudeCodeLogs:    getClaudeCodeLogs(e.config.WorkDir),
			FailureSignalType: "execution_start_failure",
		}

		// Try to get recovery guidance from analyzer
		recoveryAction, recoveryErr := e.DecideRecovery(ctx, execContext, plan)
		if recoveryErr == nil {
			result.LastOutput = fmt.Sprintf("Recovery: %s | %s", recoveryAction.Action, recoveryAction.Guidance)
			e.display.Info("Recovery", fmt.Sprintf("%s: %s", recoveryAction.Action, recoveryAction.Guidance))
		}

		result.Error = fmt.Errorf("execution failed: %w", err)
		e.display.Error(fmt.Sprintf("Execution failed: %v", err))
		result.Duration = time.Since(start)
		return result
	}
	defer reader.Close()

	// Parse the stream output, with termination callback for failure/bailout signals
	handler := llm.NewConsoleHandlerWithTerminate(e.display, cancelExec)
	if err := llm.ParseStream(reader, handler, cancelExec); err != nil {
		// Stream parsing failed - give analyzer context to decide recovery
		execContext := ExecutionContext{
			Error:             err,
			CapturedLogs:      handler.GetCapturedOutput(),
			LastToolCall:      handler.GetLastToolCall(),
			ClaudeCodeLogs:    getClaudeCodeLogs(e.config.WorkDir),
			FailureSignalType: "stream_parsing_error",
		}

		// Try to get recovery guidance from analyzer
		recoveryAction, recoveryErr := e.DecideRecovery(ctx, execContext, plan)
		if recoveryErr == nil {
			// Store recovery guidance in LastOutput for retry logic
			result.LastOutput = fmt.Sprintf("Recovery: %s | %s", recoveryAction.Action, recoveryAction.Guidance)
			e.display.Info("Recovery", fmt.Sprintf("%s: %s", recoveryAction.Action, recoveryAction.Guidance))
		}

		result.Error = fmt.Errorf("stream parsing failed: %w", err)
		e.display.Error(fmt.Sprintf("Stream parsing failed: %v", err))
		result.Duration = time.Since(start)
		return result
	}

	// Check for proper completion
	tokenStats := handler.GetTokenStats()

	// Show completion status in a Ralph box
	var statusLines []string

	if handler.HasFailed() {
		failure := handler.GetFailure()

		// For blocker failures, verify the claim before accepting it as hard failure
		if failure.Type == llm.SignalBlocked {
			blockerResult := e.RunBlockerAnalysis(ctx, failure, plan)
			if blockerResult.Error != nil {
				e.display.Warning(fmt.Sprintf("Blocker analysis failed: %v", blockerResult.Error))
			}

			if !blockerResult.IsValid {
				// Blocker is invalid - treat as soft failure with retry guidance
				result.Error = fmt.Errorf("blocker claim rejected: %s", failure.Detail)
				result.FailureType = FailureSoft
				result.LastOutput = blockerResult.Guidance // Pass guidance to retry logic
				statusLines = append(statusLines,
					fmt.Sprintf("%s Blocker claim rejected - will retry with guidance", e.display.Theme().Warning(display.SymbolWarning)),
					fmt.Sprintf("   Guidance: %s", blockerResult.Guidance))
			} else {
				// Blocker is valid - hard failure
				result.Error = fmt.Errorf("%s: %s", failure.Type, failure.Detail)
				result.TasksFailed = []string{failure.Detail}
				result.FailureType = FailureHard
				statusLines = append(statusLines,
					fmt.Sprintf("%s %s: %s (verified)", e.display.Theme().Error(display.SymbolError), failure.Type, failure.Detail))
			}
		} else {
			// Non-blocker hard failure - task/plan/build/test failed
			// Give analyzer context to suggest recovery
			execContext := ExecutionContext{
				Error:             fmt.Errorf("%s: %s", failure.Type, failure.Detail),
				CapturedLogs:      handler.GetCapturedOutput(),
				LastToolCall:      handler.GetLastToolCall(),
				ClaudeCodeLogs:    getClaudeCodeLogs(e.config.WorkDir),
				FailureSignalType: string(failure.Type),
			}

			// Try to get recovery guidance from analyzer
			recoveryAction, recoveryErr := e.DecideRecovery(ctx, execContext, plan)
			if recoveryErr == nil {
				result.LastOutput = fmt.Sprintf("Recovery: %s | %s", recoveryAction.Action, recoveryAction.Guidance)
				e.display.Info("Recovery", fmt.Sprintf("%s: %s", recoveryAction.Action, recoveryAction.Guidance))
			}

			result.Error = fmt.Errorf("%s: %s", failure.Type, failure.Detail)
			result.TasksFailed = []string{failure.Detail}
			result.FailureType = FailureHard
			statusLines = append(statusLines,
				fmt.Sprintf("%s %s: %s", e.display.Theme().Error(display.SymbolError), failure.Type, failure.Detail))
		}
	} else if handler.IsPlanComplete() {
		// Verify summary.json was created before marking success
		summaryPath := strings.Replace(plan.Path, ".json", "-summary.json", 1)
		if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
			result.Error = fmt.Errorf("plan signaled complete but summary.json not created: %s", summaryPath)
			result.FailureType = FailureSoft
			statusLines = append(statusLines,
				fmt.Sprintf("%s Plan signaled complete but summary.json missing", e.display.Theme().Warning(display.SymbolWarning)))
		} else {
			// Success - explicit completion signal with summary.json verified
			result.Success = true
			result.FailureType = FailureNone
			statusLines = append(statusLines,
				fmt.Sprintf("%s Plan complete!", e.display.Theme().Success(display.SymbolSuccess)))

			// Update state.json and roadmap.json
			if err := e.updateStateAndRoadmap(phase, plan); err != nil {
				e.display.Warning(fmt.Sprintf("Failed to update state/roadmap: %v", err))
			}

			// Commit and push all repos
			planId := fmt.Sprintf("%02d-%s", phase.Number, plan.Number)
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
			statusLines = append(statusLines,
				fmt.Sprintf("%s Bailout with progress preserved: %s", e.display.Theme().Info(display.SymbolResume), bailout.Detail),
				"   Progress section updated. Run 'ralph run' to continue.")
		} else {
			// Bailout without progress update - warn user
			result.Error = fmt.Errorf("bailout without progress update: %s", bailout.Detail)
			result.FailureType = FailureSoft
			statusLines = append(statusLines,
				fmt.Sprintf("%s Bailout WITHOUT progress update: %s", e.display.Theme().Warning(display.SymbolWarning), bailout.Detail),
				"   Warning: Progress section may not be updated. Check PLAN.md manually.")
		}
	} else if handler.ShouldBailOut() {
		// Token limit reached - Ralph's safety net triggered
		result.Error = fmt.Errorf("token limit reached: %d tokens", tokenStats.TotalTokens)
		result.FailureType = FailureSoft
		statusLines = append(statusLines,
			fmt.Sprintf("%s Token limit bailout at %d tokens", e.display.Theme().Warning(display.SymbolWarning), tokenStats.TotalTokens))
	} else {
		// No signal at all - Claude exited without any completion/failure signal
		result.FailureType = FailureSoft
		statusLines = append(statusLines,
			fmt.Sprintf("%s Claude exited without completion signal", e.display.Theme().Warning(display.SymbolWarning)),
			"   Work may be complete. Continuing to next plan...")
	}

	// Add token stats and duration
	statusLines = append(statusLines,
		fmt.Sprintf("Tokens: %d (in: %d, out: %d)", tokenStats.TotalTokens, tokenStats.InputTokens, tokenStats.OutputTokens),
		fmt.Sprintf("Duration: %s", time.Since(start).Round(time.Second)))

	e.display.RalphBox("RALPH", statusLines...)

	result.Duration = time.Since(start)
	return result
}

// ExecuteManualPlanInteractive opens an interactive Claude session for manual tasks
func (e *Executor) ExecuteManualPlanInteractive(ctx context.Context, phase *state.Phase, plan *state.Plan, start time.Time) *RunResult {
	result := &RunResult{
		PlanPath: plan.Path,
	}

	e.display.RalphBox("MANUAL PLAN",
		fmt.Sprintf("Opening interactive session: %s", plan.Name),
		"Complete manual tasks, then /exit when done")

	// Build prompt using internal executor prompt
	executorPrompt, _ := loadExecutorPrompt()
	prompt := fmt.Sprintf(`%s

## Current Plan (Manual Tasks)

Plan Location: %s

This plan contains manual tasks that require human interaction.
Guide the user through each task, verify completion, and create summary.json when done.

Begin by reading the plan and presenting the first task.`, executorPrompt, plan.Path)

	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "project.json"),
		},
		Model:   e.config.Model,
		WorkDir: e.config.WorkDir,
	}

	// Run interactively (uses --dangerously-skip-permissions)
	err := e.claude.ExecuteInteractive(ctx, opts)
	if err != nil {
		result.Error = err
		result.FailureType = FailureSoft
		e.display.Error(fmt.Sprintf("Interactive session failed: %v", err))
	} else {
		// Check if summary.json was created
		summaryPath := strings.Replace(plan.Path, ".json", "-summary.json", 1)
		if _, err := os.Stat(summaryPath); err == nil {
			result.Success = true
			result.FailureType = FailureNone
			e.display.RalphBox("MANUAL PLAN", "Manual tasks completed successfully")

			// Update state.json and roadmap.json
			if err := e.updateStateAndRoadmap(phase, plan); err != nil {
				e.display.Warning(fmt.Sprintf("Failed to update state/roadmap: %v", err))
			}

			// Commit and push all repos
			planId := fmt.Sprintf("%02d-%s", phase.Number, plan.Number)
			e.CommitAndPushRepos(planId)
		} else {
			result.FailureType = FailureSoft
			e.display.Warning("Session ended but summary.json not created - manual tasks may be incomplete")
		}
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
	e.display.LoopHeader()

	var lastPhaseNumber int = -1
	var lastPlanPath string
	var lastProgressContent string
	planRetries := make(map[string]*PlanRetryState) // Track retry state per plan

	// Determine max retries (default to iteration count)
	maxRetries := e.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = maxIterations
	}

	for i := 1; i <= maxIterations; i++ {
		// Find next plan using JSON roadmap
		phaseData, planData, err := state.FindNextPlanJSON(e.config.PlanningDir)
		if err != nil {
			return fmt.Errorf("cannot find next plan: %w", err)
		}
		if planData == nil {
			e.display.AllComplete()
			return nil
		}

		// Convert to execution-compatible structures with paths
		phase, plan := ConvertToExecutionStructs(e.config.PlanningDir, phaseData, planData)

		// Check for stuck loop - same plan found twice with NO progress made
		// This allows bailout recovery (same plan, but Progress section updated)
		if plan.Path == lastPlanPath {
			currentProgress := extractProgressSection(plan.Path)
			if currentProgress == lastProgressContent {
				// Check if we've exceeded retries
				retryState := planRetries[plan.Path]
				if retryState != nil && retryState.Attempts >= maxRetries {
					return fmt.Errorf("exceeded max retries (%d) for plan %s", maxRetries, plan.Name)
				}
				// Same progress but still have retries - will be handled below
			}
			// Progress section was updated = bailout recovery, allow continuation
			e.display.Resume(fmt.Sprintf("Resuming %s (Progress updated, continuing)", plan.Name))
		}
		lastPlanPath = plan.Path
		lastProgressContent = extractProgressSection(plan.Path)

		// Check if we're starting a new phase - if so, run phase start analysis
		if phase.Number != lastPhaseNumber {
			lastPhaseNumber = phase.Number
			// Run phase start analysis (bundles decisions AND manual tasks)
			if err := e.RunPhaseStartAnalysis(phase); err != nil {
				e.display.Warning(fmt.Sprintf("Phase start analysis failed: %v", err))
			}
			// Reload after analysis may have created new plans
			phaseData, planData, err := state.FindNextPlanJSON(e.config.PlanningDir)
			if err != nil {
				return fmt.Errorf("cannot find next plan after phase start analysis: %w", err)
			}
			if planData == nil {
				e.display.AllComplete()
				return nil
			}
			// Convert to execution-compatible structures
			phase, plan = ConvertToExecutionStructs(e.config.PlanningDir, phaseData, planData)
		}

		// Count plans from roadmap (simplified for now - will be improved in 02-03)
		total := 0
		completed := 0
		roadmap, err := state.LoadRoadmapJSON(e.config.PlanningDir)
		if err == nil {
			for _, p := range roadmap.Phases {
				total += len(p.Plans)
				// Count completed by checking JSON plan files
				// This is a temporary solution until CountPlans is updated in 02-03
			}
		}

		// Check if this is a retry and add retry info to display
		retryState := planRetries[plan.Path]
		if retryState != nil && retryState.Attempts > 0 {
			e.display.Info("Retry", fmt.Sprintf("Attempt %d/%d for %s", retryState.Attempts+1, maxRetries, plan.Name))
		}

		e.display.Iteration(i, maxIterations, plan.Name, completed, total)

		// Execute the plan (with retry guidance if this is a retry)
		result := e.ExecutePlan(ctx, phase, plan)

		// Run post-analysis ALWAYS - even on hard failures - to diagnose issues and update plans
		analysisResult := e.RunPostAnalysis(ctx, phase, plan, skipAnalysis)
		if analysisResult.Error != nil {
			e.display.Warning(fmt.Sprintf("Post-analysis failed: %v", analysisResult.Error))
		} else if analysisResult.ObservationsFound > 0 {
			e.display.Info("Analysis", fmt.Sprintf("%d observations analyzed", analysisResult.ObservationsFound))
		}

		if !result.Success {
			if result.FailureType == FailureHard {
				// Hard failure - stop the loop (analysis already ran above)
				e.display.LoopFailed(plan.Name, result.Error, completed)
				return result.Error
			}

			// Soft failure - run soft failure analysis to decide what to do
			if retryState == nil {
				retryState = &PlanRetryState{}
				planRetries[plan.Path] = retryState
			}

			if retryState.Attempts < maxRetries {
				// Run soft failure analysis
				sfResult := e.runSoftFailureAnalysis(ctx, phase, plan, result)

				switch sfResult.Decision {
				case RetryWithGuidance:
					retryState.Attempts++
					retryState.LastProgress = extractProgressSection(plan.Path)
					retryState.LastOutput = result.LastOutput
					e.display.Info("Analysis", fmt.Sprintf("Will retry: %s", sfResult.Reason))
					// Don't increment completed - this plan will be found again on next iteration
					continue
				case MarkComplete:
					// Analysis says work is done despite missing signal
					e.display.Success(fmt.Sprintf("Analysis determined complete: %s", sfResult.Reason))
				case EscalateToHuman:
					return fmt.Errorf("human intervention required: %s", sfResult.Reason)
				}
			} else {
				return fmt.Errorf("exceeded max retries (%d) for plan %s", maxRetries, plan.Name)
			}
		} else {
			// Only print checkmark on actual success
			e.display.Success(fmt.Sprintf("Complete (%s)", result.Duration.Round(time.Second)))
		}
		fmt.Println()
	}

	e.display.MaxIterations(maxIterations)
	return nil
}

func (e *Executor) buildExecutionPrompt(planPath string) string {
	// Load internal executor prompt
	executorPrompt, err := loadExecutorPrompt()
	if err != nil {
		// Fall back to inline prompt
		e.display.Warning(fmt.Sprintf("Executor prompt not available: %v (using fallback)", err))
		return e.buildFallbackExecutionPrompt(planPath)
	}

	// Add plan path context
	planContext := fmt.Sprintf(`
## Current Plan

Plan Location: %s

Execute this plan following the executor protocol above.

Begin execution now.`, planPath)

	return executorPrompt + "\n\n" + planContext
}

// buildRalphAugmentations returns Ralph-specific additions to append to GSD workflow
func buildRalphAugmentations(planPath string) string {
	return fmt.Sprintf(`---

## Ralph Augmentations

The following are Ralph-specific extensions to the GSD execution workflow.

<context>
Plan Location: %s

Ralph's analysis agent parses observations to improve subsequent plans after execution completes.
</context>

<task>
Execute the plan at the location above, following GSD execution workflow with Ralph extensions.

After completing (or failing), Ralph spawns an analysis agent to:
1. Parse all observations from this execution
2. Review subsequent plans for impact
3. Potentially restructure plans (reorder, create new, skip completed)

To maximize effectiveness:
- Record observations AS YOU FIND THEM
- Be specific about dependencies discovered
- Flag assumptions that might affect future plans
</task>

<constraints>
OBSERVATION TYPES (Ralph-specific):
- bug: Bug found that needs fixing
- stub: Stub/placeholder code that needs implementation
- api-issue: External API problem or inconsistency
- insight: Useful information for future plans
- blocker: Something blocking progress
- technical-debt: Code that works but needs improvement
- assumption: Decision made without full information
- scope-creep: Work discovered that wasn't in the plan
- dependency: Unexpected dependency between tasks/plans
- questionable: Suspicious code or pattern worth reviewing
- already-complete: Work was already done before execution
- checkpoint-automated: Checkpoint verification that was automated
- tooling-friction: Tool/environment issue that slowed progress
- test-failed: Test(s) failed during execution - enumerate test names
- test-infrastructure: Test environment issue (simulator, timeout, xcodebuild syntax)

SEVERITIES: critical, high, medium, low, info
ACTIONS: needs-fix, needs-implementation, needs-plan, needs-investigation, needs-documentation, needs-human-verify, none
</constraints>

<output-format>
OBSERVATION FORMAT (CRITICAL - Analyzer Cannot Parse Prose):

Prose observations like "## Discovery: ..." or "**Finding:** ..." CANNOT be parsed.
You MUST use this exact XML format:

`+"```"+`xml
<observation type="TYPE" severity="SEVERITY">
  <title>Short descriptive title</title>
  <detail>What you found and why it matters</detail>
  <file>path/to/relevant/file</file>
  <action>ACTION</action>
</observation>
`+"```"+`

RALPH SIGNALS:
- ###PLAN_COMPLETE### - All tasks done, verified, builds and tests pass
- ###TASK_FAILED:{name}### - A task couldn't be completed
- ###PLAN_FAILED:{check}### - Plan verification failed
- ###BUILD_FAILED:{project}### - Build failed (e.g., ios, backend)
- ###TEST_FAILED:{project}:{count}### - Tests failed that weren't failing before
- ###BLOCKED:{reason}### - Need human intervention
- ###BAILOUT:{reason}### - Preserving context, update Progress first
</output-format>

<rules>
WHAT TO OBSERVE (LOW BAR - Record Everything):

Record routine findings. Examples:
- "3 tests in X are stubs" -> type="stub", action="needs-implementation"
- "File Y has no tests" -> type="insight", action="needs-plan"
- "Function Z is deprecated but used in 5 places" -> type="technical-debt", action="needs-fix"
- "This took 30 min because docs were wrong" -> type="tooling-friction", action="needs-documentation"
- "Tests already exist for X" -> type="already-complete", action="none"
- "Found TODO comment in code" -> type="stub", action="needs-implementation"
- "API returns different format than docs say" -> type="api-issue", action="needs-investigation"

The analysis agent needs DATA to work with. Under-reporting = no analysis happens.

DOCUMENTING TEST FAILURES (CRITICAL):

Test failures require STRUCTURED observations, not prose notes. The analysis agent:
- Can detect patterns across plans (e.g., "xcodebuild issues in 4/5 plans")
- Can recommend infrastructure fixes when issues repeat
- CANNOT parse prose like "tests failed, see output"

When tests fail:
1. Use type="test-failed" (NOT generic "blocker")
2. List EACH failed test by name
3. Include error messages or expected vs actual
4. For tooling issues (xcodebuild syntax), use type="test-infrastructure"

Example - Test Failures:
`+"```"+`xml
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
`+"```"+`

Example - Test Infrastructure:
`+"```"+`xml
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
`+"```"+`

RECORDING OBSERVATIONS (Use Subagents to Save Context):

Recording observations inline burns your main context. Use Task tool to delegate:

`+"```"+`
Task(subagent_type="general-purpose", prompt="
  Add this observation to PLAN.md in the Observations section:
  <observation type=\"stub\" severity=\"medium\">
    <title>3 backend tests are stubs</title>
    <detail>image.test.ts and video.test.ts have stub tests</detail>
    <file>mix-backend/functions/src/__tests__/endpoints/</file>
    <action>needs-implementation</action>
  </observation>
")
`+"```"+`

Record observations AS YOU GO - don't batch them at the end.

PRE-EXISTING WORK HANDLING:

When you find that work in a task is ALREADY COMPLETE (files exist, code already implemented):

1. Record an observation:
   `+"```"+`xml
   <observation type="already-complete" severity="info">
     <title>Task N already implemented</title>
     <detail>The [what] already exists at [path]. Likely done in previous session.</detail>
     <file>path/to/existing/file</file>
     <action>none</action>
   </observation>
   `+"```"+`

2. Update Progress section: Mark task as `+"`[ALREADY_COMPLETE]`"+`

3. Verify existing work meets requirements - if partial, complete it

4. Continue normally: Create summary.json, signal ###PLAN_COMPLETE###

DO NOT get stuck investigating history. Document what exists and move forward.

BACKGROUND TASK VERIFICATION (MANDATORY):

BEFORE signaling ###PLAN_COMPLETE###, you MUST verify all background tasks have finished:

1. Check for running background tasks:
   - If you started tests/builds with `+"`run_in_background: true`"+`, you MUST wait for completion
   - Read the output file (from TaskOutput) to verify tests completed AND passed
   - Use Bash: `+"`ps aux | grep -E \"(xcodebuild|npm test|pytest|go test)\" | grep -v grep`"+` to check for running processes

2. You CANNOT signal ###PLAN_COMPLETE### if:
   - Background tests are still running (check process list)
   - You haven't read and verified test output shows passing
   - Build processes are still executing
   - summary.json has not been created

3. Verification sequence:
   a. Wait for all background tasks to finish (use TaskOutput with block=true)
   b. Verify test results show PASS (not just "started" or "running")
   c. Create summary.json with execution details
   d. Only then signal ###PLAN_COMPLETE###

Ralph will verify summary.json exists before accepting the completion signal.

MANUAL TASK HANDLING (AUTONOMOUS MODE - CRITICAL):

When encountering a task with type="manual" or type="checkpoint:human-action":

DO NOT wait for user input. You are running in autonomous mode without interactive input.

1. Record the task as an observation:
`+"```"+`xml
<observation type="manual-checkpoint-deferred" severity="info">
  <title>Manual task deferred: [task name]</title>
  <detail>Task requires human action. Bundled to phase-end manual plan.</detail>
  <file>[relevant file if any]</file>
  <action>none</action>
</observation>
`+"```"+`

2. Skip the task and continue to next task

3. At plan end: Note deferred manual tasks in summary.json

Why: Manual tasks are bundled into a separate XX-99-manual-PLAN.md that runs at phase end.
This keeps automation flowing while collecting human work.

Example handling:
- See task: `+"`<task type=\"manual\">Add file to Xcode...</task>`"+`
- Record observation with type="manual-checkpoint-deferred"
- Skip the task
- Continue with next auto task
- In summary.json: "Deferred 1 manual task to phase-end plan"

CONTEXT MANAGEMENT (Ralph Safety Net):

Ralph monitors your token usage and will terminate at 120K tokens as a safety net.

Self-monitoring heuristics:
- Count your tool calls: if > 50 tool calls without task completion, you're burning context
- Watch for repeated errors: 3+ retries of same fix = stuck, bail out
- File reading volume: if you've read > 20 files without progress, context is bloated

Use subagents for writing to save context:
- Use Task tool (subagent_type="general-purpose") for recording observations and progress updates
- Prompt: "Update PLAN.md with observations: [list what you found]. Update Progress section: [current state]"
- This offloads file editing work to a fresh subagent context, preserving your main context for execution

At ~100K tokens, proactively bail out:
1. Update the PLAN.md Progress section with current state
2. Update the ## Observations section with any findings
3. Document what worked, what failed, and next steps
4. Signal: ###BAILOUT:context_preservation###
</rules>

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
   - Create summary.json in the phase directory
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
   `+"```"+`xml
   <observation type="already-complete" severity="info">
     <title>Task N already implemented</title>
     <detail>The [what] already exists at [path]. Likely done in previous session.</detail>
     <file>path/to/existing/file</file>
     <action>none</action>
   </observation>
   `+"```"+`

2. **Update Progress section:** Mark task as `+"`[ALREADY_COMPLETE]`"+`

3. **Verify existing work meets requirements** - if partial, complete it

4. **Continue normally:** Create summary.json, signal ###PLAN_COMPLETE###

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

If you started ANY tasks with `+"`"+`run_in_background: true`+"`"+`, you MUST verify completion:

1. **Wait for background tasks to finish:**
   - Use TaskOutput tool with block=true to wait for completion
   - Read the output file to verify results

2. **Check no processes are still running:**
   - Use Bash: `+"`"+`ps aux | grep -E "(xcodebuild|npm test|pytest|go test)" | grep -v grep`+"`"+`
   - Empty output = no running processes

3. **Verify test results show PASS:**
   - Read the test output file
   - Look for "PASS" or "succeeded" (not just "started" or "running")
   - Count actual test results, not just the command starting

4. **Create summary.json BEFORE signaling:**
   - Ralph will verify summary.json exists before accepting the completion signal
   - If summary.json is missing, your ###PLAN_COMPLETE### will be rejected

**Verification sequence:**
a. Wait for all background tasks (TaskOutput with block=true)
b. Verify test output shows actual PASS results
c. Create summary.json with execution details
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

// SoftFailureDecision indicates how to handle a soft failure
type SoftFailureDecision int

const (
	RetryWithGuidance SoftFailureDecision = iota
	MarkComplete
	EscalateToHuman
)

// SoftFailureAnalysisResult holds the result of analyzing a soft failure
type SoftFailureAnalysisResult struct {
	Decision SoftFailureDecision
	Reason   string
	Guidance string
}

// runSoftFailureAnalysis evaluates what happened during a soft failure and decides how to proceed
func (e *Executor) runSoftFailureAnalysis(ctx context.Context, phase *state.Phase, plan *state.Plan, result *RunResult) *SoftFailureAnalysisResult {
	// Check if summary.json exists → likely complete
	summaryPath := strings.Replace(plan.Path, ".json", "-summary.json", 1)
	if _, err := os.Stat(summaryPath); err == nil {
		return &SoftFailureAnalysisResult{
			Decision: MarkComplete,
			Reason:   "summary.json exists, plan appears complete",
		}
	}

	// Check Progress section
	progress := extractProgressSection(plan.Path)
	if strings.Contains(progress, "[COMPLETE]") && !strings.Contains(progress, "[PENDING]") {
		// All tasks marked complete but no summary
		return &SoftFailureAnalysisResult{
			Decision: RetryWithGuidance,
			Reason:   "Tasks complete but summary.json missing",
			Guidance: "All tasks appear complete. Create summary.json and signal ###PLAN_COMPLETE###",
		}
	}

	// Check for manual checkpoint in last output using TaskType enum
	if containsHumanActionTask(result.LastOutput) {
		return &SoftFailureAnalysisResult{
			Decision: RetryWithGuidance,
			Reason:   "Hit manual checkpoint - will skip on retry",
			Guidance: "Skip manual tasks (they're bundled to phase-end plan). Continue with auto tasks only.",
		}
	}

	// Default: retry with general guidance
	return &SoftFailureAnalysisResult{
		Decision: RetryWithGuidance,
		Reason:   "Unexpected exit - retrying with progress logging",
	}
}

// RunPhaseStartAnalysis runs phase-start analysis to bundle decisions and manual tasks
func (e *Executor) RunPhaseStartAnalysis(phase *state.Phase) error {
	// Create decisions plan (runs FIRST - plan 00)
	created, err := e.MaybeCreateDecisionsPlan(phase)
	if err != nil {
		e.display.Warning(fmt.Sprintf("Failed to create decisions plan: %v", err))
	} else if created {
		e.display.Info("Phase Start", "Created decisions plan (runs first)")
	}

	// Create manual tasks plan (runs LAST - plan 99)
	created, err = e.MaybeCreateManualTasksPlan(phase)
	if err != nil {
		e.display.Warning(fmt.Sprintf("Failed to create manual tasks plan: %v", err))
	} else if created {
		e.display.Info("Phase Start", "Created manual tasks plan (runs last)")
	}

	return nil
}

// buildRetryGuidance generates progressive guidance for retry attempts
func buildRetryGuidance(state *PlanRetryState) string {
	return fmt.Sprintf(`
### RETRY ATTEMPT %d

This plan has been attempted %d times previously.

**CRITICAL: Log progress BEFORE each action.**
Before executing any task, update the ## Progress section with:
- "Task N: [STARTING] - About to [what you're doing]"
Then execute. Then update to [COMPLETE] or [FAILED].

This ensures we capture state even if execution is interrupted.

**Previous attempt info:**
Last progress state:
%s

Last output before exit:
%s

Analyze what went wrong and proceed carefully.
`, state.Attempts+1, state.Attempts, state.LastProgress, state.LastOutput)
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
	PlanNumber  string // String to support decimal plan numbers like "5.1"
	PlanName    string
	PlanPath    string
	TaskContent string
	Context     string
}

// MaybeCreateDecisionsPlan scans a phase for checkpoint:decision tasks and creates a bundled decisions plan
func (e *Executor) MaybeCreateDecisionsPlan(phase *state.Phase) (bool, error) {
	// Check if decisions plan already exists
	decisionsPath := filepath.Join(phase.Path, fmt.Sprintf("%02d-00.json", phase.Number))
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

	e.display.Info("Decisions", fmt.Sprintf("Found %d checkpoints, creating decisions plan...", len(decisions)))

	// Create the decisions plan
	err := e.createDecisionsPlan(phase, decisions, decisionsPath)
	if err != nil {
		return false, err
	}

	e.display.Info("Decisions", fmt.Sprintf("Created %s", filepath.Base(decisionsPath)))

	return true, nil
}

// createDecisionsPlan generates the bundled decisions plan file in JSON format
func (e *Executor) createDecisionsPlan(phase *state.Phase, decisions []DecisionCheckpoint, outPath string) error {
	// Build tasks from decision checkpoints
	var tasks []types.Task
	for i, d := range decisions {
		tasks = append(tasks, types.Task{
			ID:     fmt.Sprintf("decision-%d", i+1),
			Name:   fmt.Sprintf("Decision from Plan %s", d.PlanName),
			Type:   types.TaskTypeManual, // Decisions require human input
			Files:  []string{d.PlanPath},
			Action: d.TaskContent,
			Done:   "Decision made and recorded",
			Status: types.StatusPending,
		})
	}

	// Build verification items
	verification := []string{
		"All decisions have been made",
		"Each decision is recorded in state.json",
		"Decision rationales are documented",
	}

	plan := &types.Plan{
		Phase:        fmt.Sprintf("%02d-%s", phase.Number, phase.Name),
		PlanNumber:   "00",
		Status:       types.StatusPending,
		Objective:    fmt.Sprintf("Make all architectural and approach decisions before executing Phase %d plans", phase.Number),
		Tasks:        tasks,
		Verification: verification,
		CreatedAt:    time.Now(),
	}

	return state.SavePlanJSON(outPath, plan)
}

// ManualTask represents a manual task extracted from a plan
type ManualTask struct {
	PlanNumber  string
	PlanName    string
	PlanPath    string
	TaskName    string
	TaskContent string
}

// MaybeCreateManualTasksPlan scans a phase for manual and checkpoint:human-action tasks and creates a bundled manual tasks plan
func (e *Executor) MaybeCreateManualTasksPlan(phase *state.Phase) (bool, error) {
	// Check if manual tasks plan already exists
	manualPath := filepath.Join(phase.Path, fmt.Sprintf("%02d-99.json", phase.Number))
	if _, err := os.Stat(manualPath); err == nil {
		// Manual tasks plan already exists
		return false, nil
	}

	// Scan all plans in this phase for tasks that require human action
	var manualTasks []ManualTask
	// Match task with type attribute for extraction
	taskWithTypePattern := regexp.MustCompile(`(?s)(<task\s+[^>]*>)(.*?)(</task>)`)

	for _, plan := range phase.Plans {
		content, err := os.ReadFile(plan.Path)
		if err != nil {
			continue
		}

		// Find all tasks and check their types using the TaskType enum
		matches := taskWithTypePattern.FindAllStringSubmatch(string(content), -1)
		for i, match := range matches {
			if len(match) > 3 {
				taskOpenTag := match[1] // <task type="...">
				taskContent := match[2] // task content

				// Extract and validate task type
				taskType, err := ExtractTaskType(taskOpenTag)
				if err != nil {
					// Log invalid task type but continue
					e.display.Warning(fmt.Sprintf("Invalid task type in %s: %v", plan.Name, err))
					continue
				}

				// Only bundle tasks that require human action
				if taskType.RequiresHumanAction() {
					manualTasks = append(manualTasks, ManualTask{
						PlanNumber:  plan.Number,
						PlanName:    plan.Name,
						PlanPath:    plan.Path,
						TaskName:    fmt.Sprintf("Manual Task %d", i+1),
						TaskContent: strings.TrimSpace(taskContent),
					})
				}
			}
		}
	}

	if len(manualTasks) == 0 {
		// No manual tasks found in this phase
		return false, nil
	}

	e.display.Info("Manual Tasks", fmt.Sprintf("Found %d manual tasks, creating plan...", len(manualTasks)))

	// Create the manual tasks plan
	err := e.createManualTasksPlan(phase, manualTasks, manualPath)
	if err != nil {
		return false, err
	}

	e.display.Info("Manual Tasks", fmt.Sprintf("Created %s", filepath.Base(manualPath)))

	return true, nil
}

// createManualTasksPlan generates the bundled manual tasks plan file in JSON format
func (e *Executor) createManualTasksPlan(phase *state.Phase, manualTasks []ManualTask, outPath string) error {
	// Build tasks from manual task checkpoints
	var tasks []types.Task
	for i, mt := range manualTasks {
		tasks = append(tasks, types.Task{
			ID:     fmt.Sprintf("manual-%d", i+1),
			Name:   fmt.Sprintf("%s (from Plan %s)", mt.TaskName, mt.PlanName),
			Type:   types.TaskTypeManual,
			Files:  []string{mt.PlanPath},
			Action: mt.TaskContent,
			Done:   "Manual task completed and verified",
			Status: types.StatusPending,
		})
	}

	// Build verification items
	verification := []string{
		"All manual tasks completed",
		"Verified each task's done criteria",
	}

	plan := &types.Plan{
		Phase:        fmt.Sprintf("%02d-%s", phase.Number, phase.Name),
		PlanNumber:   "99",
		Status:       types.StatusPending,
		Objective:    fmt.Sprintf("Complete all manual tasks that couldn't be automated during Phase %d execution", phase.Number),
		Tasks:        tasks,
		Verification: verification,
		CreatedAt:    time.Now(),
	}

	return state.SavePlanJSON(outPath, plan)
}

// ConvertToExecutionStructs converts JSON types to execution-compatible structs with paths
func ConvertToExecutionStructs(planningDir string, phaseData *types.Phase, planData *types.Plan) (*state.Phase, *state.Plan) {
	// Build phase directory path
	phaseDir := filepath.Join(planningDir, "phases",
		fmt.Sprintf("%02d-%s", phaseData.Number, slugify(phaseData.Name)))

	// Build plan path (format: {phase}-{plan}.json)
	planPath := filepath.Join(phaseDir,
		fmt.Sprintf("%02d-%s.json", phaseData.Number, planData.PlanNumber))

	// Extract plan name from objective or use default
	planName := extractPlanName(planData.Objective)

	phase := &state.Phase{
		Number: phaseData.Number,
		Name:   phaseData.Name,
		Path:   phaseDir,
		// Plans field can be populated if needed
	}

	// Determine plan type from tasks (check if any are manual)
	planType := state.PlanTypeExecute // Default to execute
	for _, task := range planData.Tasks {
		if task.Type == types.TaskTypeManual {
			planType = state.PlanTypeManual
			break
		}
	}

	plan := &state.Plan{
		Number: planData.PlanNumber,
		Name:   planName,
		Path:   planPath,
		Type:   planType,
	}

	return phase, plan
}

// slugify converts a name to a directory-safe slug
func slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	result := ""
	for _, c := range slug {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result += string(c)
		}
	}
	return result
}

// extractPlanName extracts a short name from the plan objective
func extractPlanName(objective string) string {
	// Take first sentence or first 80 chars
	lines := strings.Split(objective, ".")
	if len(lines) > 0 {
		name := strings.TrimSpace(lines[0])
		if len(name) > 80 {
			name = name[:77] + "..."
		}
		return name
	}
	return objective
}

// updateStateAndRoadmap updates state.json and roadmap.json after plan completion
func (e *Executor) updateStateAndRoadmap(phase *state.Phase, plan *state.Plan) error {
	// Update state.json
	projectState, err := state.LoadStateJSON(e.config.PlanningDir)
	if err != nil {
		return fmt.Errorf("cannot load state.json: %w", err)
	}

	// Update current phase and last updated timestamp
	projectState.CurrentPhase = phase.Number
	projectState.LastUpdated = time.Now()

	if err := state.SaveStateJSON(e.config.PlanningDir, projectState); err != nil {
		return fmt.Errorf("cannot save state.json: %w", err)
	}

	// Update the plan JSON file to mark it complete
	// Build plan JSON path (format: {phase-dir}/{phase}-{plan}.json)
	phaseDir := filepath.Join(e.config.PlanningDir, "phases",
		fmt.Sprintf("%02d-%s", phase.Number, slugify(phase.Name)))
	planJSONPath := filepath.Join(phaseDir,
		fmt.Sprintf("%02d-%s.json", phase.Number, plan.Number))

	// Load plan JSON
	planJSON, err := state.LoadPlanJSON(planJSONPath)
	if err != nil {
		return fmt.Errorf("cannot load plan JSON: %w", err)
	}

	// Mark as complete
	now := time.Now()
	planJSON.Status = types.StatusComplete
	planJSON.CompletedAt = &now

	if err := state.SavePlanJSON(planJSONPath, planJSON); err != nil {
		return fmt.Errorf("cannot save plan JSON: %w", err)
	}

	// Check if all plans in this phase are complete
	// Load all plans for the phase
	plans, err := state.LoadAllPlansJSON(phaseDir)
	if err != nil {
		return fmt.Errorf("cannot load all plans for phase: %w", err)
	}

	// Check if all are complete
	allComplete := true
	for i := range plans {
		if plans[i].Status != types.StatusComplete {
			allComplete = false
			break
		}
	}

	// If all plans complete, mark phase as complete in roadmap
	if allComplete {
		roadmap, err := state.LoadRoadmapJSON(e.config.PlanningDir)
		if err != nil {
			return fmt.Errorf("cannot load roadmap.json: %w", err)
		}

		// Find and update the phase status
		for i := range roadmap.Phases {
			if roadmap.Phases[i].Number == phase.Number {
				roadmap.Phases[i].Status = types.StatusComplete
				break
			}
		}

		if err := state.SaveRoadmapJSON(e.config.PlanningDir, roadmap); err != nil {
			return fmt.Errorf("cannot save roadmap.json: %w", err)
		}
	}

	return nil
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

	for _, repo := range repos {
		// Check if there are changes to commit
		statusCmd := exec.Command("git", "-C", repo, "status", "--porcelain")
		output, _ := statusCmd.Output()
		if len(output) == 0 {
			continue // No changes in this repo
		}

		repoName := filepath.Base(repo)
		e.display.Info("Git", fmt.Sprintf("Committing changes in %s", repoName))

		// Stage all changes
		addCmd := exec.Command("git", "-C", repo, "add", "-A")
		if err := addCmd.Run(); err != nil {
			e.display.Warning(fmt.Sprintf("Failed to stage in %s: %v", repoName, err))
			continue
		}

		// Commit with plan reference
		commitMsg := fmt.Sprintf("chore(%s): auto-commit after plan completion", planId)
		commitCmd := exec.Command("git", "-C", repo, "commit", "-m", commitMsg)
		if err := commitCmd.Run(); err != nil {
			e.display.Warning(fmt.Sprintf("Failed to commit in %s: %v", repoName, err))
			continue
		}

		// Push to current branch
		pushCmd := exec.Command("git", "-C", repo, "push")
		if err := pushCmd.Run(); err != nil {
			e.display.Warning(fmt.Sprintf("Failed to push %s: %v", repoName, err))
			continue
		}

		e.display.Success(fmt.Sprintf("Pushed %s", repoName))
	}

	return nil
}
