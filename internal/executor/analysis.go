package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/state"
	"github.com/fatih/color"
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
	yellow := color.New(color.FgYellow).SprintFunc()

	if skipAnalysis {
		fmt.Printf("[%s] %s Skipped (--skip-analysis flag)\n",
			time.Now().Format("15:04:05"), yellow("Analysis:"))
		return result
	}

	// Read the completed plan to extract observations
	planContent, err := os.ReadFile(plan.Path)
	if err != nil {
		result.Error = fmt.Errorf("cannot read plan for analysis: %w", err)
		return result
	}

	observations := ParseObservations(string(planContent))

	// Also check SUMMARY.md for prose-format observations
	// The execution agent often writes observations there under "Auto-fixed Issues"
	summaryPath := strings.Replace(plan.Path, "-PLAN.md", "-SUMMARY.md", 1)
	if summaryContent, err := os.ReadFile(summaryPath); err == nil {
		summaryObs := ParseSummaryObservations(string(summaryContent))
		observations = append(observations, summaryObs...)
	}

	result.ObservationsFound = len(observations)

	if len(observations) == 0 {
		fmt.Printf("[%s] %s No observations to analyze\n",
			time.Now().Format("15:04:05"), yellow("Analysis:"))
		return result
	}

	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("[%s] %s Found %d observations, running analysis...\n",
		time.Now().Format("15:04:05"), cyan("Analysis:"), len(observations))

	// Find subsequent plans in this phase and future phases
	subsequentPlans := e.findSubsequentPlans(phase, plan)
	if len(subsequentPlans) == 0 {
		fmt.Printf("[%s] %s No subsequent plans to analyze\n",
			time.Now().Format("15:04:05"), yellow("Analysis:"))
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

	// Parse the stream output
	handler := llm.NewConsoleHandler()
	if err := llm.ParseStream(reader, handler); err != nil {
		result.Error = fmt.Errorf("analysis stream parsing failed: %w", err)
		return result
	}

	// Count modified plans by checking git status or similar
	// For now, we trust the analysis agent updated what was needed
	fmt.Printf("[%s] %s Analysis complete\n",
		time.Now().Format("15:04:05"), cyan("Analysis:"))

	// Check if phase is complete and needs verification plan
	created, err := e.MaybeCreateVerificationPlan(phase)
	if err != nil {
		fmt.Printf("[%s] %s Failed to create verification plan: %v\n",
			time.Now().Format("15:04:05"), yellow("⚠"), err)
	} else if created {
		result.NewPlansCreated++
	}

	return result
}

// ParseObservations extracts observation blocks from PLAN.md content
// Also supports legacy <discovery> tags for backward compatibility
func ParseObservations(content string) []Observation {
	yellow := color.New(color.FgYellow).SprintFunc()

	// Check for prose observations that can't be parsed
	prosePattern := regexp.MustCompile(`(?i)(##\s*Discovery:|##\s*Observation:|\*\*Discovery\*\*:|\*\*Finding\*\*:|\[Discovery)`)
	if prosePattern.MatchString(content) {
		fmt.Printf("[%s] %s Found prose observations - these cannot be parsed!\n",
			time.Now().Format("15:04:05"), yellow("⚠ Warning:"))
		fmt.Printf("           Use XML format: <observation type=\"...\">...</observation>\n")
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
` + "```" + `markdown
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
` + "```" + `

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
	PlanNumber     int
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

		observations := ParseObservations(string(content))
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
		if plan.Number == 0 || plan.Number >= 99 {
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
	cyan := color.New(color.FgCyan).SprintFunc()

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
		fmt.Printf("[%s] %s Phase complete with no pending verifications\n",
			time.Now().Format("15:04:05"), cyan("Verification:"))
		return false, nil
	}

	fmt.Printf("[%s] %s Found %d checkpoints requiring verification, creating plan...\n",
		time.Now().Format("15:04:05"), cyan("Verification:"), len(verifications))

	// Create the verification plan
	err := e.createVerificationPlan(phase, verifications, verificationPath)
	if err != nil {
		return false, err
	}

	fmt.Printf("[%s] %s Created %s\n",
		time.Now().Format("15:04:05"), cyan("Verification:"), filepath.Base(verificationPath))

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

**From plan:** %s (Plan %02d-%02d)
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
