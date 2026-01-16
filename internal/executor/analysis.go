package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/daydemir/ralph/internal/display"
	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/state"
)

// Observation represents a finding captured during plan execution
type Observation struct {
	Type     string // bug, stub, api-issue, insight, blocker, technical-debt, etc.
	Severity string // critical, high, medium, low, info
	Title    string
	Detail   string
	File     string
	Action   string // needs-fix, needs-implementation, needs-plan, needs-investigation, none
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
func (e *Executor) RunPostAnalysis(ctx context.Context, phase *state.Phase, plan *state.Plan, skipAnalysis bool) *AnalysisResult {
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

	// Also check SUMMARY.md for prose-format observations
	// The execution agent often writes observations there under "Auto-fixed Issues"
	summaryPath := strings.Replace(plan.Path, "-PLAN.md", "-SUMMARY.md", 1)
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

	// Build the analysis prompt
	prompt := e.buildAnalysisPrompt(plan, observations, subsequentPlans)

	// Execute analysis with Claude - includes Write tool for plan creation/restructuring
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "ROADMAP.md"),
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
// Also supports legacy <discovery> tags for backward compatibility
func ParseObservations(content string, disp *display.Display) []Observation {
	// Check for prose observations that can't be parsed
	prosePattern := regexp.MustCompile(`(?i)(##\s*Discovery:|##\s*Observation:|\*\*Discovery\*\*:|\*\*Finding\*\*:|\[Discovery)`)
	if prosePattern.MatchString(content) {
		if disp != nil {
			disp.Warning("Found prose observations - these cannot be parsed! Use XML format.")
		}
	}

	var observations []Observation

	// Match both <observation> and <discovery> blocks (for backward compat)
	pattern := regexp.MustCompile(`(?s)<(observation|discovery)\s+type="([^"]+)"\s+severity="([^"]+)">\s*` +
		`<title>([^<]+)</title>\s*` +
		`<detail>([^<]+)</detail>\s*` +
		`(?:<file>([^<]*)</file>\s*)?` +
		`<action>([^<]+)</action>\s*` +
		`</(observation|discovery)>`)

	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 8 {
			o := Observation{
				Type:     strings.TrimSpace(match[2]),
				Severity: strings.TrimSpace(match[3]),
				Title:    strings.TrimSpace(match[4]),
				Detail:   strings.TrimSpace(match[5]),
				File:     strings.TrimSpace(match[6]),
				Action:   strings.TrimSpace(match[7]),
			}
			observations = append(observations, o)
		}
	}

	return observations
}

// ParseSummaryObservations extracts observations from SUMMARY.md prose format
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
		var detail, file string

		// Issue line
		if issueMatch := regexp.MustCompile(`(?m)-\s*\*\*Issue:\*\*\s*(.+)`).FindStringSubmatch(block); len(issueMatch) > 1 {
			detail = strings.TrimSpace(issueMatch[1])
		}

		// Fix line (append to detail)
		if fixMatch := regexp.MustCompile(`(?m)-\s*\*\*Fix:\*\*\s*(.+)`).FindStringSubmatch(block); len(fixMatch) > 1 {
			if detail != "" {
				detail += " | Fix: " + strings.TrimSpace(fixMatch[1])
			} else {
				detail = "Fix: " + strings.TrimSpace(fixMatch[1])
			}
		}

		// Files modified
		if fileMatch := regexp.MustCompile(`(?m)-\s*\*\*Files modified:\*\*\s*` + "`?" + `([^` + "`" + `\n]+)` + "`?").FindStringSubmatch(block); len(fileMatch) > 1 {
			file = strings.TrimSpace(fileMatch[1])
		}

		// Determine action based on context
		action := "none" // Auto-fixed issues are already resolved
		if strings.Contains(strings.ToLower(block), "pending") || strings.Contains(strings.ToLower(block), "blocker") {
			action = "needs-fix"
		}

		// Map type strings to standard types
		switch obsType {
		case "bug":
			obsType = "bug"
		case "code fix", "code-fix":
			obsType = "bug"
		default:
			// Keep as-is or default to insight
			if obsType == "" {
				obsType = "insight"
			}
		}

		observations = append(observations, Observation{
			Type:     obsType,
			Severity: "medium", // Auto-fixed issues are typically medium
			Title:    title,
			Detail:   detail,
			File:     file,
			Action:   action,
		})
	}

	return observations
}

// findSubsequentPlans returns paths to plans that come after the current one
func (e *Executor) findSubsequentPlans(currentPhase *state.Phase, currentPlan *state.Plan) []string {
	var subsequent []string

	phases, err := state.LoadPhases(e.config.PlanningDir)
	if err != nil {
		return subsequent
	}

	foundCurrent := false
	for _, phase := range phases {
		for _, plan := range phase.Plans {
			if plan.Path == currentPlan.Path {
				foundCurrent = true
				continue
			}
			if foundCurrent && !plan.IsCompleted {
				subsequent = append(subsequent, plan.Path)
			}
		}
	}

	return subsequent
}

// buildAnalysisPrompt creates the prompt for the post-run analysis agent
func (e *Executor) buildAnalysisPrompt(plan *state.Plan, observations []Observation, subsequentPlans []string) string {
	var observationsText strings.Builder
	for i, o := range observations {
		observationsText.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n", i+1, o.Type, o.Severity, o.Title))
		observationsText.WriteString(fmt.Sprintf("   Detail: %s\n", o.Detail))
		if o.File != "" {
			observationsText.WriteString(fmt.Sprintf("   File: %s\n", o.File))
		}
		observationsText.WriteString(fmt.Sprintf("   Action: %s\n\n", o.Action))
	}

	return fmt.Sprintf(`You are analyzing observations from a completed plan execution.

## Just-Completed Plan
%s

## Observations from Execution
%s

## Subsequent Plans to Review
%s

## Your Task

Review each observation and determine its impact on subsequent plans.

### Observation Types and Actions

**High-impact types for plan restructuring:**
- **assumption**: A decision was made without full information - check if subsequent plans rely on this assumption
- **scope-creep**: Work was discovered that wasn't in any plan - may require new plans
- **dependency**: An unexpected dependency was found - may require plan reordering
- **questionable**: Suspicious code was found - add review notes to relevant plans

**Standard types:**
- **bug**, **stub**, **api-issue**: May need fixes before dependent plans proceed
- **technical-debt**, **tooling-friction**, **env-discovery**: Document for future reference
- **insight**, **blocker**: May affect how subsequent tasks are approached

### Plan Restructuring Authority

You have FULL AUTHORITY to restructure the plan sequence based on observations. This includes:

#### 1. REORDER PLANS
If observations show Plan X depends on Plan Y (but Y comes after X), reorder the sequence:
- Renumber plan files to reflect the new order (e.g., if Plan 05 must come before Plan 03, rename files accordingly)
- Update ROADMAP.md to show the new sequence
- Document the reason for reordering

Example:
  Before: Plan 1 -> Plan 2 -> Plan 3 -> Plan 4 -> Plan 5
  Observation: Plan 3 depends on Plan 5
  After:  Plan 1 -> Plan 2 -> Plan 5 -> Plan 3 -> Plan 4

#### 2. CREATE NEW PLANS
If observations reveal work not covered by any existing plan:
- Create new XX-PLAN.md files using the standard plan template
- Insert at the appropriate position in the sequence
- Update ROADMAP.md to include the new plan
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
- Mark the plan as SKIPPED in ROADMAP.md with reason
- Document evidence of completion (files that exist, tests that pass, etc.)
- Remove from active execution queue (but keep original file for reference)

#### 4. UPDATE ROADMAP.md
ALL restructuring changes MUST be reflected in ROADMAP.md:
- Reordering: Update phase/plan sequence
- New plans: Add entry at appropriate position
- Skipped plans: Mark with SKIPPED status and reason

### Action Guidelines

For observations with action "needs-fix", "needs-implementation", or "needs-plan":
1. Read the relevant subsequent plan files
2. Determine if the observation:
   - Invalidates tasks in a plan (work is already done, or approach is wrong) -> SKIP the plan
   - Means a dependency must be resolved first -> REORDER plans
   - Requires work not covered by any plan -> CREATE new plan
   - Suggests plan order should change -> REORDER plans

For observations with action "needs-documentation":
1. Suggest updates to CLAUDE.md or project documentation
2. Note which tooling friction or environment observations should be captured
3. Add context to plans if the documentation affects their execution

For observations with action "needs-investigation":
1. Add investigation notes to relevant plans
2. Flag assumptions that need verification before proceeding
3. Consider creating a new investigation plan if scope is significant

### Test Failure Pattern Analysis

When reviewing test-related observations (test-failed, test-infrastructure, tooling-friction):

1. **Count occurrences**: If same test infrastructure issue appears in 2+ plans:
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
- ROADMAP.md serves as audit trail
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
`, plan.Path, observationsText.String(), strings.Join(subsequentPlans, "\n"))
}

// HasActionableObservations returns true if any observations require action
func HasActionableObservations(observations []Observation) bool {
	for _, o := range observations {
		switch o.Action {
		case "needs-fix", "needs-implementation", "needs-plan", "needs-investigation", "needs-documentation":
			return true
		}
	}
	return false
}

// FilterBySeverity returns observations at or above the given severity
func FilterBySeverity(observations []Observation, minSeverity string) []Observation {
	severityOrder := map[string]int{
		"critical": 5,
		"high":     4,
		"medium":   3,
		"low":      2,
		"info":     1,
	}

	minLevel := severityOrder[minSeverity]
	var filtered []Observation
	for _, o := range observations {
		if severityOrder[o.Severity] >= minLevel {
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

// CollectCheckpointObservations scans all completed plans in a phase for checkpoint-automated observations
func (e *Executor) CollectCheckpointObservations(phase *state.Phase) []CheckpointVerification {
	var verifications []CheckpointVerification

	for _, plan := range phase.Plans {
		// Only check completed plans (have SUMMARY.md)
		if !plan.IsCompleted {
			continue
		}

		content, err := os.ReadFile(plan.Path)
		if err != nil {
			continue
		}

		observations := ParseObservations(string(content), nil)
		for _, o := range observations {
			if o.Type == "checkpoint-automated" && o.Action == "needs-human-verify" {
				verification := CheckpointVerification{
					PlanNumber:     plan.Number,
					PlanName:       plan.Name,
					PlanPath:       plan.Path,
					CheckpointName: o.Title,
					AutomatedTest:  o.File,
				}

				// Parse detail for automated/needs-human breakdown
				lines := strings.Split(o.Detail, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Automated aspects:") {
						verification.WhatAutomated = append(verification.WhatAutomated,
							strings.TrimSpace(strings.TrimPrefix(line, "Automated aspects:")))
					} else if strings.HasPrefix(line, "Still needs human review:") {
						verification.NeedsHuman = append(verification.NeedsHuman,
							strings.TrimSpace(strings.TrimPrefix(line, "Still needs human review:")))
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
func (e *Executor) IsPhaseComplete(phase *state.Phase) bool {
	for _, plan := range phase.Plans {
		// Skip special plans (decisions and verification)
		num, _ := strconv.ParseFloat(plan.Number, 64)
		if num == 0 || num >= 99 {
			continue
		}
		if !plan.IsCompleted {
			return false
		}
	}
	return true
}

// MaybeCreateVerificationPlan checks if phase is complete and creates bundled verification plan
func (e *Executor) MaybeCreateVerificationPlan(phase *state.Phase) (bool, error) {
	// Check if verification plan already exists
	verificationPath := filepath.Join(phase.Path, fmt.Sprintf("%02d-99-verification-PLAN.md", phase.Number))
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

// createVerificationPlan generates the bundled verification plan file
func (e *Executor) createVerificationPlan(phase *state.Phase, verifications []CheckpointVerification, outPath string) error {
	var content strings.Builder

	// Write frontmatter
	content.WriteString(fmt.Sprintf(`---
phase: %d
plan: 99
type: verification
status: pending
---

# Phase %d Verification: Human Checkpoint Review

## Objective

Review all automated verifications from this phase and provide feedback.
This is the final quality gate before the phase is considered complete.

## Checkpoints to Review

`, phase.Number, phase.Number))

	// Write each checkpoint
	for i, v := range verifications {
		content.WriteString(fmt.Sprintf(`### %d. %s

**From plan:** %s (Plan %02d-%s)
`, i+1, v.CheckpointName, v.PlanName, phase.Number, v.PlanNumber))

		if v.AutomatedTest != "" {
			content.WriteString(fmt.Sprintf("**Automated test:** `%s`\n", v.AutomatedTest))
		}

		if len(v.WhatAutomated) > 0 {
			content.WriteString("**What was automated:**\n")
			for _, item := range v.WhatAutomated {
				content.WriteString(fmt.Sprintf("- %s\n", item))
			}
		}

		if len(v.NeedsHuman) > 0 {
			content.WriteString("**Still needs human review:**\n")
			for _, item := range v.NeedsHuman {
				content.WriteString(fmt.Sprintf("- %s\n", item))
			}
		}

		content.WriteString("\n**How to verify:**\n")
		content.WriteString("1. Review the automated test results\n")
		content.WriteString("2. Manually check the aspects that couldn't be automated\n")
		content.WriteString("3. Provide approval or describe issues\n\n")
	}

	// Write verification process
	content.WriteString(`## Verification Process

1. Review each checkpoint above
2. For each checkpoint, respond:
   - ✅ **Approved** - Verification passes
   - ❌ **Issue: [description]** - Describe what's wrong

3. Issues will trigger fix plan creation

<task type="checkpoint:human-action">
Review all checkpoints above and provide feedback for each.
<verify>User has reviewed and provided feedback for all checkpoints</verify>
</task>

## Post-Verification

After all checkpoints are reviewed:
- **If all approved**: Phase is complete, proceed to next phase
- **If issues found**: Create fix plans for each issue, then re-verify

## Success Criteria

- All automated tests pass
- All manual verification aspects reviewed
- Human approval received for each checkpoint
- Any issues documented and addressed
`)

	return os.WriteFile(outPath, []byte(content.String()), 0644)
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
func (e *Executor) RunBlockerAnalysis(ctx context.Context, failure *llm.FailureSignal, plan *state.Plan) *BlockerAnalysisResult {
	result := &BlockerAnalysisResult{}

	e.display.Info("Blocker Analysis", fmt.Sprintf("Verifying blocker claim: %s", failure.Detail))

	// Build the blocker analysis prompt
	prompt := buildBlockerAnalysisPrompt(failure.Detail, plan.Path)

	// Execute analysis with Claude using Opus for better research capability
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "PROJECT.md"),
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
func (e *Executor) DecideRecovery(ctx context.Context, execCtx ExecutionContext, plan *state.Plan) (*RecoveryAction, error) {
	e.display.Info("Recovery Analysis", "Analyzing execution failure context")

	// Build prompt with error context + logs from multiple sources
	prompt := buildRecoveryPrompt(execCtx, plan)

	// Execute recovery analysis with Claude
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "PROJECT.md"),
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
func buildRecoveryPrompt(execCtx ExecutionContext, plan *state.Plan) string {
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
