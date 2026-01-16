package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/daydemir/ralph/internal/display"
	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/state"
	"github.com/daydemir/ralph/internal/types"
	"github.com/daydemir/ralph/internal/utils"
)

// Observation represents a finding captured during plan execution
// Simplified: Running agents observe, analyzer infers severity and decides actions
type Observation struct {
	Type        string // blocker, finding, or completion
	Title       string // Short descriptive title
	Description string // What was noticed and agent's thoughts
	File        string // Where (optional)
}

// ExecutionContext holds context about a failed execution for recovery analysis
type ExecutionContext struct {
	Error             error
	CapturedLogs      []string // Everything Claude output before failing
	LastToolCall      string   // What tool was Claude trying to use
	ClaudeCodeLogs    string   // Fallback: Claude Code's own conversation logs
	FailureSignalType string   // Type of failure signal (task_failed, blocked, etc.)
}

// RecoveryAction represents what to do after a failed execution
type RecoveryAction struct {
	Action   string // retry, fix-state, break-into-chunks, skip, manual-intervention
	Guidance string // Specific guidance on how to proceed
	Reason   string // Why this action was chosen
}

// AnalysisResult holds the result of post-run analysis
type AnalysisResult struct {
	ObservationsFound int
	PlansModified     int
	NewPlansCreated   int
	Error             error
}

// RunPostAnalysis spawns an agent to analyze observations and potentially update subsequent plans
// If execCtx is provided and contains an error, the error context is included in the analysis prompt
func (e *Executor) RunPostAnalysis(ctx context.Context, phase *types.Phase, plan *types.Plan, skipAnalysis bool, execCtx *ExecutionContext) *AnalysisResult {
	result := &AnalysisResult{}

	if skipAnalysis {
		e.display.Info("Analysis", "Skipped (--skip-analysis flag)")
		return result
	}

	// Read the completed plan to extract observations
	planContent, err := os.ReadFile(plan.Path)
	if err != nil {
		result.Error = fmt.Errorf("cannot read plan for analysis: %w", err)
		return result
	}

	observations := ParseObservations(string(planContent), e.display)

	// Also check summary.json for prose-format observations
	// The execution agent often writes observations there under "Auto-fixed Issues"
	summaryPath := strings.Replace(plan.Path, ".json", "-summary.json", 1)
	if summaryContent, err := os.ReadFile(summaryPath); err == nil {
		summaryObs := ParseSummaryObservations(string(summaryContent))
		observations = append(observations, summaryObs...)
	}

	result.ObservationsFound = len(observations)

	if len(observations) == 0 {
		e.display.Analysis("No observations to analyze")
		return result
	}

	// Show analysis start with observation count
	e.display.AnalysisStart(len(observations))

	// Find subsequent plans in this phase and future phases
	subsequentPlans := e.findSubsequentPlans(phase, plan)
	if len(subsequentPlans) == 0 {
		e.display.Analysis("No subsequent plans to analyze")
		return result
	}

	// Build the analysis prompt (include execution context if available)
	prompt := e.buildAnalysisPrompt(plan, observations, subsequentPlans, execCtx)

	// Execute analysis with Claude - includes Write tool for plan creation/restructuring
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "roadmap.json"),
		},
		Model: e.config.Model,
		AllowedTools: []string{
			"Read", "Write", "Edit", "Glob", "Grep", "Bash",
		},
		WorkDir: e.config.WorkDir,
	}

	reader, err := e.claude.Execute(ctx, opts)
	if err != nil {
		result.Error = fmt.Errorf("analysis execution failed: %w", err)
		return result
	}
	defer reader.Close()

	// Parse the stream output (no termination callback - analysis should run to completion)
	handler := llm.NewConsoleHandlerWithDisplay(e.display)
	if err := llm.ParseStream(reader, handler, nil); err != nil {
		result.Error = fmt.Errorf("analysis stream parsing failed: %w", err)
		return result
	}

	// Count modified plans by checking git status or similar
	// For now, we trust the analysis agent updated what was needed
	e.display.AnalysisComplete(result.PlansModified, result.NewPlansCreated)

	// Check if phase is complete and needs verification plan
	created, err := e.MaybeCreateVerificationPlan(phase)
	if err != nil {
		e.display.Warning(fmt.Sprintf("Failed to create verification plan: %v", err))
	} else if created {
		result.NewPlansCreated++
	}

	return result
}

// ParseObservations extracts observation blocks from PLAN.md content
// Supports both new simplified format and legacy format for backward compatibility
func ParseObservations(content string, disp *display.Display) []Observation {
	// Check for prose observations that can't be parsed
	prosePattern := regexp.MustCompile(`(?i)(##\s*Discovery:|##\s*Observation:|\*\*Discovery\*\*:|\*\*Finding\*\*:|\[Discovery)`)
	if prosePattern.MatchString(content) {
		if disp != nil {
			disp.Warning("Found prose observations - these cannot be parsed! Use XML format.")
		}
	}

	var observations []Observation

	// New simplified format: <observation type="TYPE"><title>...</title><description>...</description><file>...</file></observation>
	newPattern := regexp.MustCompile(`(?s)<observation\s+type="([^"]+)">\s*` +
		`<title>([^<]+)</title>\s*` +
		`<description>([^<]*)</description>\s*` +
		`(?:<file>([^<]*)</file>\s*)?` +
		`</observation>`)

	matches := newPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 4 {
			o := Observation{
				Type:        strings.TrimSpace(match[1]),
				Title:       strings.TrimSpace(match[2]),
				Description: strings.TrimSpace(match[3]),
				File:        "",
			}
			if len(match) >= 5 {
				o.File = strings.TrimSpace(match[4])
			}
			observations = append(observations, o)
		}
	}

	// Legacy format for backward compatibility: <observation type="TYPE" severity="SEVERITY">...<detail>...<action>...</observation>
	legacyPattern := regexp.MustCompile(`(?s)<(observation|discovery)\s+type="([^"]+)"\s+severity="([^"]+)">\s*` +
		`<title>([^<]+)</title>\s*` +
		`<detail>([^<]+)</detail>\s*` +
		`(?:<file>([^<]*)</file>\s*)?` +
		`<action>([^<]+)</action>\s*` +
		`</(observation|discovery)>`)

	legacyMatches := legacyPattern.FindAllStringSubmatch(content, -1)
	for _, match := range legacyMatches {
		if len(match) >= 8 {
			// Convert legacy format to new format
			o := Observation{
				Type:        strings.TrimSpace(match[2]),
				Title:       strings.TrimSpace(match[4]),
				Description: strings.TrimSpace(match[5]),
				File:        strings.TrimSpace(match[6]),
			}
			observations = append(observations, o)
		}
	}

	return observations
}

// ParseSummaryObservations extracts observations from summary.json prose format
// Looks for patterns in "Auto-fixed Issues" and "Issues Encountered" sections
// Format: **N. [Rule X - TYPE] TITLE** followed by bullet points with details
func ParseSummaryObservations(content string) []Observation {
	var observations []Observation

	// Pattern for auto-fixed issues: **N. [Rule X - TYPE] TITLE**
	// Example: **1. [Rule 1 - Bug] iOS 18 availability required for SpatialAudioComponent**
	issuePattern := regexp.MustCompile(`(?m)\*\*\d+\.\s*\[Rule\s+\d+\s*-\s*([^\]]+)\]\s*([^\*]+)\*\*`)

	// Find all matches
	matches := issuePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		obsType := strings.ToLower(strings.TrimSpace(match[1]))
		title := strings.TrimSpace(match[2])

		// Find the position of this match to extract following bullet points
		matchPos := strings.Index(content, match[0])
		if matchPos == -1 {
			continue
		}

		// Extract the block after the title (until next ## or **N. pattern)
		blockStart := matchPos + len(match[0])
		blockEnd := len(content)

		// Find next section boundary
		nextSection := regexp.MustCompile(`(?m)^(##|\*\*\d+\.)`)
		if loc := nextSection.FindStringIndex(content[blockStart:]); loc != nil {
			blockEnd = blockStart + loc[0]
		}

		block := content[blockStart:blockEnd]

		// Extract details from bullet points
		var description, file string

		// Issue line
		if issueMatch := regexp.MustCompile(`(?m)-\s*\*\*Issue:\*\*\s*(.+)`).FindStringSubmatch(block); len(issueMatch) > 1 {
			description = strings.TrimSpace(issueMatch[1])
		}

		// Fix line (append to description)
		if fixMatch := regexp.MustCompile(`(?m)-\s*\*\*Fix:\*\*\s*(.+)`).FindStringSubmatch(block); len(fixMatch) > 1 {
			if description != "" {
				description += " | Fix: " + strings.TrimSpace(fixMatch[1])
			} else {
				description = "Fix: " + strings.TrimSpace(fixMatch[1])
			}
		}

		// Files modified
		if fileMatch := regexp.MustCompile(`(?m)-\s*\*\*Files modified:\*\*\s*` + "`?" + `([^` + "`" + `\n]+)` + "`?").FindStringSubmatch(block); len(fileMatch) > 1 {
			file = strings.TrimSpace(fileMatch[1])
		}

		// Map legacy type strings to new simplified types
		// Most auto-fixed issues are "findings" that were resolved
		mappedType := "finding"
		if strings.Contains(strings.ToLower(block), "blocker") {
			mappedType = "blocker"
		} else if strings.Contains(strings.ToLower(obsType), "complete") || strings.Contains(strings.ToLower(obsType), "done") {
			mappedType = "completion"
		}

		observations = append(observations, Observation{
			Type:        mappedType,
			Title:       title,
			Description: description,
			File:        file,
		})
	}

	return observations
}

// findSubsequentPlans returns paths to plans that come after the current one
func (e *Executor) findSubsequentPlans(currentPhase *types.Phase, currentPlan *types.Plan) []string {
	var subsequent []string

	// Nil check to prevent panic when currentPlan is nil
	if currentPlan == nil {
		return subsequent
	}

	roadmap, err := state.LoadRoadmapJSON(e.config.PlanningDir)
	if err != nil {
		return subsequent
	}

	foundCurrent := false
	for _, p := range roadmap.Phases {
		phaseDir := filepath.Join(e.config.PlanningDir, "phases",
			fmt.Sprintf("%02d-%s", p.Number, utils.Slugify(p.Name)))

		// Load all plans for this phase from disk
		plans, err := state.LoadAllPlansJSON(phaseDir)
		if err != nil {
			continue
		}

		for _, plan := range plans {
			planPath := filepath.Join(phaseDir, fmt.Sprintf("%02d-%s.json", p.Number, plan.PlanNumber))
			if planPath == currentPlan.Path {
				foundCurrent = true
				continue
			}
			if foundCurrent && plan.Status != types.StatusComplete {
				subsequent = append(subsequent, planPath)
			}
		}
	}

	return subsequent
}

// buildAnalysisPrompt creates the prompt for the post-run analysis agent
func (e *Executor) buildAnalysisPrompt(plan *types.Plan, observations []Observation, subsequentPlans []string, execCtx *ExecutionContext) string {
	var observationsText strings.Builder
	for i, o := range observations {
		observationsText.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, o.Type, o.Title))
		observationsText.WriteString(fmt.Sprintf("   Description: %s\n", o.Description))
		if o.File != "" {
			observationsText.WriteString(fmt.Sprintf("   File: %s\n", o.File))
		}
		observationsText.WriteString("\n")
	}

	// Build execution error section if context contains an error
	var executionErrorSection string
	if execCtx != nil && execCtx.Error != nil {
		executionErrorSection = fmt.Sprintf(`

## Execution Error

The execution was interrupted with an error. This context is crucial for understanding what happened:

**Error:** %s
**Failure Type:** %s
**Last Tool Call:** %s

### Captured Logs (last output before failure)
%s

This error context should inform your analysis:
- If the error is a stream parsing issue (e.g., "token too long"), the execution may have been doing something that generated large output
- If the error is a tool execution failure, check if subsequent plans might hit the same issue
- Consider whether this error indicates a systemic problem that needs addressing

`, execCtx.Error, execCtx.FailureSignalType, execCtx.LastToolCall, strings.Join(execCtx.CapturedLogs, "\n"))
	}

	return fmt.Sprintf(`You are analyzing observations from a completed plan execution.%s

## Just-Completed Plan
%s

## Observations from Execution
%s

## Subsequent Plans to Review
%s

## Your Task

Review each observation and determine its impact on subsequent plans.

### Observation Types (Simplified)

There are only 3 observation types - you decide severity and actions from context:

- **blocker**: Agent couldn't continue without human intervention - investigate why
- **finding**: Something interesting was noticed - analyze the description to decide impact
- **completion**: Work was already done or not needed - may allow skipping plans

Your job is to analyze the **description** field to understand:
- What category of issue this is (bug, stub, dependency, scope-creep, etc.)
- How severe it is (critical, high, medium, low)
- What action is needed (needs-fix, needs-plan, none, etc.)

### Plan Restructuring Authority

You have FULL AUTHORITY to restructure the plan sequence based on observations. This includes:

#### 1. REORDER PLANS
If observations show Plan X depends on Plan Y (but Y comes after X), reorder the sequence:
- Renumber plan files to reflect the new order (e.g., if Plan 05 must come before Plan 03, rename files accordingly)
- Update roadmap.json to show the new sequence
- Document the reason for reordering

Example:
  Before: Plan 1 -> Plan 2 -> Plan 3 -> Plan 4 -> Plan 5
  Observation: Plan 3 depends on Plan 5
  After:  Plan 1 -> Plan 2 -> Plan 5 -> Plan 3 -> Plan 4

#### 2. CREATE NEW PLANS
If observations reveal work not covered by any existing plan:
- Create new XX-plan.json files using the standard plan template
- Insert at the appropriate position in the sequence
- Update roadmap.json to include the new plan
- Set status to PENDING

New plan template structure:
`+"```"+`markdown
---
phase: [phase-number]
plan: [plan-number]
status: pending
---

# Phase [X] Plan [Y]: [Name]

## Objective
[What this plan accomplishes - derived from observation]

## Context
Created by analysis agent based on observation:
- Type: [observation type]
- From: [original plan path]
- Detail: [observation detail]

## Tasks
<task type="auto">
[Task description]
<verify>[Verification command]</verify>
</task>

## Verification
- [ ] [Verification criteria]

## Success Criteria
- [Criteria]
`+"```"+`

#### 3. SKIP/REMOVE PLANS
If observations show planned work is already complete:
- Mark the plan as SKIPPED in roadmap.json with reason
- Document evidence of completion (files that exist, tests that pass, etc.)
- Remove from active execution queue (but keep original file for reference)

#### 4. UPDATE roadmap.json
ALL restructuring changes MUST be reflected in roadmap.json:
- Reordering: Update phase/plan sequence
- New plans: Add entry at appropriate position
- Skipped plans: Mark with SKIPPED status and reason

### Analysis Guidelines

For each observation, analyze the description to determine impact:

**Blockers** - Investigate whether they're legitimate:
1. Search for similar issues that were solved before
2. Check if workarounds exist
3. If truly blocked, document why human action is required
4. If solvable, add guidance to subsequent plans

**Findings** - Determine what action is needed:
1. Read the description to understand the category (bug, stub, dependency, etc.)
2. Decide if it:
   - Invalidates tasks in a plan (work is already done, or approach is wrong) -> SKIP the plan
   - Means a dependency must be resolved first -> REORDER plans
   - Requires work not covered by any plan -> CREATE new plan
   - Just needs documentation -> Note for CLAUDE.md update

**Completions** - Verify and update plans:
1. Check if subsequent plans can be skipped
2. Document evidence of completion
3. Update roadmap accordingly

### Test Failure Pattern Analysis

When reviewing findings that describe test failures (look for keywords like "test failed", "xcodebuild", "npm test"):

1. **Count occurrences**: If same test infrastructure issue appears in 2+ observations:
   - Create a new "fix-test-infrastructure" plan to resolve it once
   - Example: "xcodebuild syntax issues in 3 plans -> create plan to document working patterns in CLAUDE.md"

2. **Check for systemic issues**:
   - Repeated simulator crashes -> plan to fix simulator setup
   - Repeated xcodebuild errors -> plan to document working commands
   - Repeated timeout issues -> plan to optimize test configuration

3. **Recommend infrastructure plans** when test tooling wastes >60 minutes total across plans

### Blocker Observation Verification

When reviewing observations with type="blocker":

1. **Challenge the blocker claim**: Search for evidence that the blocker may not be legitimate:
   - Search .planning/archive/progress-*.txt for similar issues that were solved
   - Search the codebase for workarounds or alternative approaches
   - Check if the blocker is actually a misunderstanding of requirements

2. **If blocker appears invalid**: Add guidance to subsequent plans on how to work around it:
   - Document the workaround approach in the plan's <context> section
   - Reference any codebase examples that show the solution pattern
   - Flag for retry with specific guidance

3. **If blocker is legitimate**: Document why it truly requires human action:
   - Requires credentials or physical device access
   - Depends on genuinely unavailable external systems
   - Requires permissions that cannot be obtained programmatically

4. **Update subsequent plans** that may hit the same blocker with preemptive guidance

For each plan that needs updating:
1. Add a note in the plan's <context> section referencing the observation
2. If a task is invalidated, add a note explaining why
3. If a blocker exists, add a <blocker> tag at the top

## Safety Considerations
- All restructuring is logged in execution history
- Original plans are preserved (renamed, not deleted)
- roadmap.json serves as audit trail
- Document the observation that triggered each change

## Rules
- Only restructure if observations directly warrant it
- Do NOT modify the just-completed plan (only subsequent plans)
- Keep changes minimal and targeted to the observation
- Flag critical assumptions and scope-creep that may need human review
- Commit changes with message: "chore(analysis): restructure plans based on [plan] observations"

## Completion
When done analyzing, output a brief summary:
- Number of plans reviewed
- Number of plans modified
- Plans reordered (if any)
- New plans created (if any)
- Plans skipped (if any)
- Any critical issues that need immediate attention

Signal completion with: ###ANALYSIS_COMPLETE###
`, executionErrorSection, plan.Path, observationsText.String(), strings.Join(subsequentPlans, "\n"))
}

// HasActionableObservations returns true if any observations are blockers or findings
// (completions don't require action)
func HasActionableObservations(observations []Observation) bool {
	for _, o := range observations {
		if o.Type == "blocker" || o.Type == "finding" {
			return true
		}
	}
	return false
}

// FilterByType returns observations of a specific type
func FilterByType(observations []Observation, obsType string) []Observation {
	var filtered []Observation
	for _, o := range observations {
		if o.Type == obsType {
			filtered = append(filtered, o)
		}
	}
	return filtered
}

// CheckpointVerification represents a checkpoint that needs human verification
type CheckpointVerification struct {
	PlanNumber     string // String to support decimal plan numbers like "5.1"
	PlanName       string
	PlanPath       string
	CheckpointName string
	AutomatedTest  string   // Path to automated test if created
	WhatAutomated  []string // What aspects were automated
	NeedsHuman     []string // What still needs human review
}

// CollectCheckpointObservations scans all completed plans in a phase for observations that need human verification
// Now looks for "finding" observations with descriptions mentioning verification needed
func (e *Executor) CollectCheckpointObservations(phase *types.Phase) []CheckpointVerification {
	var verifications []CheckpointVerification

	// Load all plans for this phase from disk
	plans, err := state.LoadAllPlansJSON(phase.Path)
	if err != nil {
		return verifications
	}

	for _, plan := range plans {
		// Only check completed plans
		if plan.Status != types.StatusComplete {
			continue
		}

		planPath := filepath.Join(phase.Path, fmt.Sprintf("%02d-%s.json", phase.Number, plan.PlanNumber))
		content, err := os.ReadFile(planPath)
		if err != nil {
			continue
		}

		planName := utils.ExtractPlanName(plan.Objective)

		observations := ParseObservations(string(content), nil)
		for _, o := range observations {
			// Look for findings that mention verification or human review needed
			if o.Type == "finding" && (strings.Contains(strings.ToLower(o.Description), "needs human") ||
				strings.Contains(strings.ToLower(o.Description), "verification needed") ||
				strings.Contains(strings.ToLower(o.Description), "human review")) {
				verification := CheckpointVerification{
					PlanNumber:     plan.PlanNumber,
					PlanName:       planName,
					PlanPath:       planPath,
					CheckpointName: o.Title,
					AutomatedTest:  o.File,
				}

				// Parse description for automated/needs-human breakdown
				lines := strings.Split(o.Description, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Automated aspects:") || strings.HasPrefix(line, "Automated:") {
						verification.WhatAutomated = append(verification.WhatAutomated,
							strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Automated aspects:"), "Automated:")))
					} else if strings.HasPrefix(line, "Still needs human review:") || strings.HasPrefix(line, "Needs human:") {
						verification.NeedsHuman = append(verification.NeedsHuman,
							strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Still needs human review:"), "Needs human:")))
					}
				}

				verifications = append(verifications, verification)
			}
		}
	}

	return verifications
}

// IsPhaseComplete checks if all regular plans in the phase are completed
// (excluding decision and verification plans)
func (e *Executor) IsPhaseComplete(phase *types.Phase) bool {
	// Load all plans for this phase from disk
	plans, err := state.LoadAllPlansJSON(phase.Path)
	if err != nil {
		return false
	}

	for _, plan := range plans {
		// Skip special plans (decisions and verification)
		num, _ := strconv.ParseFloat(plan.PlanNumber, 64)
		if num == 0 || num >= 99 {
			continue
		}
		if plan.Status != types.StatusComplete {
			return false
		}
	}
	return true
}

// MaybeCreateVerificationPlan checks if phase is complete and creates bundled verification plan
func (e *Executor) MaybeCreateVerificationPlan(phase *types.Phase) (bool, error) {
	// Check if verification plan already exists
	verificationPath := filepath.Join(phase.Path, fmt.Sprintf("%02d-99.json", phase.Number))
	if _, err := os.Stat(verificationPath); err == nil {
		// Verification plan already exists
		return false, nil
	}

	// Check if phase is complete (all regular plans done)
	if !e.IsPhaseComplete(phase) {
		return false, nil
	}

	// Collect all checkpoint observations that need human verification
	verifications := e.CollectCheckpointObservations(phase)
	if len(verifications) == 0 {
		// No verifications needed
		e.display.Info("Verification", "Phase complete with no pending verifications")
		return false, nil
	}

	e.display.Info("Verification", fmt.Sprintf("Found %d checkpoints requiring verification, creating plan...", len(verifications)))

	// Create the verification plan
	err := e.createVerificationPlan(phase, verifications, verificationPath)
	if err != nil {
		return false, err
	}

	e.display.Info("Verification", fmt.Sprintf("Created %s", filepath.Base(verificationPath)))

	return true, nil
}

// createVerificationPlan generates the bundled verification plan file in JSON format
func (e *Executor) createVerificationPlan(phase *types.Phase, verifications []CheckpointVerification, outPath string) error {
	// Build tasks from checkpoint verifications
	var tasks []types.Task
	for i, v := range verifications {
		// Build action description with checkpoint details
		actionDetails := fmt.Sprintf("Checkpoint: %s\nFrom plan: %s\n", v.CheckpointName, v.PlanName)
		if v.AutomatedTest != "" {
			actionDetails += fmt.Sprintf("Automated test: %s\n", v.AutomatedTest)
		}
		if len(v.WhatAutomated) > 0 {
			actionDetails += "Automated: " + strings.Join(v.WhatAutomated, ", ") + "\n"
		}
		if len(v.NeedsHuman) > 0 {
			actionDetails += "Needs human review: " + strings.Join(v.NeedsHuman, ", ") + "\n"
		}

		tasks = append(tasks, types.Task{
			ID:     fmt.Sprintf("verify-%d", i+1),
			Name:   fmt.Sprintf("Verify: %s", v.CheckpointName),
			Type:   types.TaskTypeManual, // Verification requires human action
			Files:  []string{v.PlanPath},
			Action: actionDetails,
			Done:   "Human has reviewed and approved this checkpoint",
			Status: types.StatusPending,
		})
	}

	// Build verification items
	verification := []string{
		"All automated tests pass",
		"All manual verification aspects reviewed",
		"Human approval received for each checkpoint",
		"Any issues documented and addressed",
	}

	plan := &types.Plan{
		Phase:        fmt.Sprintf("%02d-%s", phase.Number, phase.Name),
		PlanNumber:   "99",
		Status:       types.StatusPending,
		Objective:    fmt.Sprintf("Review all automated verifications from Phase %d and provide feedback", phase.Number),
		Tasks:        tasks,
		Verification: verification,
		CreatedAt:    time.Now(),
	}

	return state.SavePlanJSON(outPath, plan)
}

// BlockerAnalysisResult holds the result of blocker verification
type BlockerAnalysisResult struct {
	IsValid  bool   // true if blocker is legitimate, false if it can be worked around
	Guidance string // guidance for retry if blocker is invalid
	Error    error
}

// buildBlockerAnalysisPrompt creates the prompt for verifying a blocker claim
func buildBlockerAnalysisPrompt(blockerClaim string, planPath string) string {
	return fmt.Sprintf(`You are a blocker verification agent. An execution agent claimed it was blocked, but we need to verify this claim before accepting it.

## Blocker Claim
**Reason:** %s
**Plan:** %s

## Your Task

Verify whether this blocker is legitimate or if it can be worked around.

### Step 1: Search Historical Progress

Search for similar issues that were solved before:
- Use Glob to find: .planning/archive/progress-*.txt
- Use Grep to search these files for keywords from the blocker claim
- Look for patterns like "was blocked by" followed by "solved by" or "worked around"

### Step 2: Search Codebase for Solutions

Search the codebase for existing solutions:
- Use Grep to search for relevant code patterns, error messages, or workarounds
- Check CLAUDE.md for documented solutions to common issues
- Look for TODO comments or documentation about the blocked functionality
- Search for similar implementations that might provide a pattern to follow

### Step 3: Analyze the Plan

Read the plan file to understand:
- What task was being attempted
- What the expected outcome was
- Whether there are alternative approaches mentioned

### Step 4: Make a Decision

Based on your research, determine if the blocker is:

**VALID** - The blocker is legitimate if:
- It requires human action (e.g., credentials, physical device, manual approval)
- It depends on external systems that are genuinely unavailable
- It requires resources or permissions that cannot be obtained programmatically
- No historical solutions or workarounds exist for this type of issue

**INVALID** - The blocker can be worked around if:
- Similar issues were solved in historical progress files
- The codebase contains patterns or solutions that apply
- Alternative approaches exist that weren't tried
- The issue is a misunderstanding of requirements or capabilities
- Documentation provides guidance that wasn't followed

## Output Format

After your investigation, output EXACTLY ONE of these signals:

If the blocker is legitimate:
###BLOCKER_VALID:{brief reason why it's truly blocked}###

If the blocker can be worked around:
###BLOCKER_INVALID:{specific guidance on how to proceed}###

## Rules
- Be thorough in your search before deciding
- Look for at least 3 different search patterns before concluding
- Favor finding workarounds over accepting blockers
- If uncertain, lean toward INVALID with guidance to try alternative approaches
- Keep guidance actionable and specific

Begin investigation now.`, blockerClaim, planPath)
}

// RunBlockerAnalysis verifies whether a blocker claim is legitimate
func (e *Executor) RunBlockerAnalysis(ctx context.Context, failure *llm.FailureSignal, plan *types.Plan) *BlockerAnalysisResult {
	result := &BlockerAnalysisResult{}

	e.display.Info("Blocker Analysis", fmt.Sprintf("Verifying blocker claim: %s", failure.Detail))

	// Build the blocker analysis prompt
	prompt := buildBlockerAnalysisPrompt(failure.Detail, plan.Path)

	// Execute analysis with Claude using Opus for better research capability
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "project.json"),
		},
		Model: "opus", // Use Opus for better reasoning on blocker verification
		AllowedTools: []string{
			"Read", "Glob", "Grep", "Bash",
		},
		WorkDir: e.config.WorkDir,
	}

	reader, err := e.claude.Execute(ctx, opts)
	if err != nil {
		result.Error = fmt.Errorf("blocker analysis execution failed: %w", err)
		result.IsValid = true // Default to valid if analysis fails
		return result
	}
	defer reader.Close()

	// Parse the stream output and capture the decision
	handler := llm.NewConsoleHandlerWithDisplay(e.display)

	// We need to capture the output to find the BLOCKER_VALID/INVALID signal
	var outputBuilder strings.Builder
	customHandler := &blockerAnalysisHandler{
		ConsoleHandler: handler,
		outputBuilder:  &outputBuilder,
	}

	if err := llm.ParseStream(reader, customHandler, nil); err != nil {
		result.Error = fmt.Errorf("blocker analysis stream parsing failed: %w", err)
		result.IsValid = true // Default to valid if parsing fails
		return result
	}

	// Parse the output for the decision signal
	output := outputBuilder.String()

	validPattern := regexp.MustCompile(`###BLOCKER_VALID:([^#]+)###`)
	invalidPattern := regexp.MustCompile(`###BLOCKER_INVALID:([^#]+)###`)

	if match := validPattern.FindStringSubmatch(output); len(match) > 1 {
		result.IsValid = true
		result.Guidance = strings.TrimSpace(match[1])
		e.display.Info("Blocker Analysis", fmt.Sprintf("Blocker confirmed valid: %s", result.Guidance))
	} else if match := invalidPattern.FindStringSubmatch(output); len(match) > 1 {
		result.IsValid = false
		result.Guidance = strings.TrimSpace(match[1])
		e.display.Info("Blocker Analysis", fmt.Sprintf("Blocker can be worked around: %s", result.Guidance))
	} else {
		// No clear signal - default to valid to be safe
		result.IsValid = true
		result.Guidance = "No clear determination from analysis"
		e.display.Warning("Blocker analysis did not produce a clear decision - treating as valid")
	}

	return result
}

// blockerAnalysisHandler wraps ConsoleHandler to capture output text
type blockerAnalysisHandler struct {
	*llm.ConsoleHandler
	outputBuilder *strings.Builder
}

func (h *blockerAnalysisHandler) OnText(text string) {
	h.outputBuilder.WriteString(text)
	h.ConsoleHandler.OnText(text)
}

func (h *blockerAnalysisHandler) OnDone(result string) {
	h.outputBuilder.WriteString(result)
	h.ConsoleHandler.OnDone(result)
}

// DecideRecovery analyzes execution failure context and decides how to proceed
func (e *Executor) DecideRecovery(ctx context.Context, execCtx ExecutionContext, plan *types.Plan) (*RecoveryAction, error) {
	e.display.Info("Recovery Analysis", "Analyzing execution failure context")

	// Build prompt with error context + logs from multiple sources
	prompt := buildRecoveryPrompt(execCtx, plan)

	// Execute recovery analysis with Claude
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "project.json"),
		},
		Model: e.config.Model,
		AllowedTools: []string{
			"Read", "Glob", "Grep", "Bash",
		},
		WorkDir: e.config.WorkDir,
	}

	reader, err := e.claude.Execute(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("recovery analysis execution failed: %w", err)
	}
	defer reader.Close()

	// Parse the stream output and capture the decision
	handler := llm.NewConsoleHandlerWithDisplay(e.display)

	var outputBuilder strings.Builder
	customHandler := &recoveryAnalysisHandler{
		ConsoleHandler: handler,
		outputBuilder:  &outputBuilder,
	}

	if err := llm.ParseStream(reader, customHandler, nil); err != nil {
		return nil, fmt.Errorf("recovery analysis stream parsing failed: %w", err)
	}

	// Parse the output for recovery decision signals
	output := outputBuilder.String()

	return parseRecoveryDecision(output, e.display)
}

// buildRecoveryPrompt creates the prompt for recovery decision analysis
func buildRecoveryPrompt(execCtx ExecutionContext, plan *types.Plan) string {
	// Join captured logs with newlines
	logsText := strings.Join(execCtx.CapturedLogs, "\n")

	// Truncate logs if too long (keep last 10KB for context)
	if len(logsText) > 10000 {
		logsText = "...[truncated]...\n" + logsText[len(logsText)-10000:]
	}

	claudeCodeLogsSection := ""
	if execCtx.ClaudeCodeLogs != "" {
		claudeCodeLogsSection = fmt.Sprintf(`
Claude Code conversation logs (fallback source):
%s
`, execCtx.ClaudeCodeLogs)
	}

	return fmt.Sprintf(`You are a recovery decision agent. An execution just failed, and you need to decide how to proceed.

## Execution Failure

**Error:** %s
**Failure Type:** %s
**Last Tool Called:** %s
**Plan:** %s

## Captured Output

Ralph captured logs (Claude's output before failure):
%s
%s

## Your Task

Analyze the failure context and decide what to do next.

### Recovery Options

1. **RETRY** - Try the same task again (with optional guidance)
   - Use when: Transient error, network issue, timing problem
   - Signal: ###RECOVERY:retry:{guidance}###

2. **FIX_STATE** - Fix a corrupted file or state first, then retry
   - Use when: File is corrupted, state is inconsistent, bad data written
   - Signal: ###RECOVERY:fix-state:{what to fix}###

3. **BREAK_CHUNKS** - Break the work into smaller pieces
   - Use when: Task is too large, context overflow, complexity issue
   - Signal: ###RECOVERY:break-chunks:{how to split}###

4. **SKIP** - Skip this task and continue
   - Use when: Task is optional, blocker is external, not critical for progress
   - Signal: ###RECOVERY:skip:{reason}###

5. **MANUAL** - Needs human intervention
   - Use when: Requires credentials, external approval, human decision
   - Signal: ###RECOVERY:manual:{what needs human action}###

### Analysis Guidelines

Look for these patterns in the captured logs:

**Stream parsing errors:**
- "bufio.Scanner: token too long" → likely large output, may need different approach
- "unexpected EOF" → connection lost, retry may work
- JSON parsing errors → output format issue, may need guidance

**Tool execution errors:**
- "command not found" → missing dependency, needs fix-state
- "permission denied" → needs credentials or manual intervention
- "timeout" → may need smaller chunks or retry

**Claude's own errors:**
- "I tried X but Y happened" → Claude documented the issue, extract guidance
- Repeated failed attempts → may need different approach or break into chunks
- "I'm blocked by Z" → analyze if truly blocked or can be worked around

### Output Format

After analysis, output EXACTLY ONE recovery signal:

###RECOVERY:{action}:{guidance}###

Where action is one of: retry, fix-state, break-chunks, skip, manual

Example signals:
- ###RECOVERY:retry:Use --force flag to bypass cache###
- ###RECOVERY:fix-state:Delete corrupted .planning/STATE.md and regenerate###
- ###RECOVERY:break-chunks:Split into 3 smaller tasks: auth, validation, response###
- ###RECOVERY:skip:Test requires GPU which is not available###
- ###RECOVERY:manual:Need API credentials for external service###

Begin analysis now.
`, execCtx.Error, execCtx.FailureSignalType, execCtx.LastToolCall, plan.Path, logsText, claudeCodeLogsSection)
}

// parseRecoveryDecision extracts recovery action from analyzer output
func parseRecoveryDecision(output string, disp *display.Display) (*RecoveryAction, error) {
	// Pattern: ###RECOVERY:{action}:{guidance}###
	pattern := regexp.MustCompile(`###RECOVERY:([^:]+):([^#]+)###`)

	match := pattern.FindStringSubmatch(output)
	if len(match) < 3 {
		return nil, fmt.Errorf("no recovery decision found in analyzer output")
	}

	action := strings.TrimSpace(match[1])
	guidance := strings.TrimSpace(match[2])

	// Validate action
	validActions := map[string]bool{
		"retry":        true,
		"fix-state":    true,
		"break-chunks": true,
		"skip":         true,
		"manual":       true,
	}

	if !validActions[action] {
		return nil, fmt.Errorf("invalid recovery action: %s", action)
	}

	disp.Info("Recovery Decision", fmt.Sprintf("Action: %s | Guidance: %s", action, guidance))

	return &RecoveryAction{
		Action:   action,
		Guidance: guidance,
		Reason:   fmt.Sprintf("Analyzer decided: %s", guidance),
	}, nil
}

// recoveryAnalysisHandler wraps ConsoleHandler to capture output text
type recoveryAnalysisHandler struct {
	*llm.ConsoleHandler
	outputBuilder *strings.Builder
}

func (h *recoveryAnalysisHandler) OnText(text string) {
	h.outputBuilder.WriteString(text)
	h.ConsoleHandler.OnText(text)
}

func (h *recoveryAnalysisHandler) OnDone(result string) {
	h.outputBuilder.WriteString(result)
	h.ConsoleHandler.OnDone(result)
}

// getClaudeCodeLogs attempts to read Claude Code's conversation logs as a fallback
// Returns empty string if logs cannot be found or read
func getClaudeCodeLogs(workDir string) string {
	// Claude Code stores conversation logs in ~/.claude/projects/<project>/conversations/
	// Try to find the most recent conversation log

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Get project name from workDir (basename)
	projectName := filepath.Base(workDir)
	conversationsDir := filepath.Join(homeDir, ".claude", "projects", projectName, "conversations")

	// Check if directory exists
	if _, err := os.Stat(conversationsDir); os.IsNotExist(err) {
		return ""
	}

	// Find most recent conversation file
	entries, err := os.ReadDir(conversationsDir)
	if err != nil {
		return ""
	}

	if len(entries) == 0 {
		return ""
	}

	// Get the most recent file (they're typically timestamped)
	var mostRecent os.DirEntry
	var mostRecentTime int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > mostRecentTime {
			mostRecentTime = info.ModTime().Unix()
			mostRecent = entry
		}
	}

	if mostRecent == nil {
		return ""
	}

	// Read the log file (limit to last 50KB to avoid huge context)
	logPath := filepath.Join(conversationsDir, mostRecent.Name())
	content, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}

	// Truncate to last 50KB if larger
	if len(content) > 50000 {
		content = content[len(content)-50000:]
		return "...[truncated]...\n" + string(content)
	}

	return string(content)
}
