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

// Discovery represents a finding captured during plan execution
type Discovery struct {
	Type     string // bug, stub, api-issue, insight, blocker, technical-debt
	Severity string // critical, high, medium, low, info
	Title    string
	Detail   string
	File     string
	Action   string // needs-fix, needs-implementation, needs-plan, needs-investigation, none
}

// AnalysisResult holds the result of post-run analysis
type AnalysisResult struct {
	DiscoveriesFound int
	PlansModified    int
	NewPlansCreated  int
	Error            error
}

// RunPostAnalysis spawns an agent to analyze discoveries and potentially update subsequent plans
func (e *Executor) RunPostAnalysis(ctx context.Context, phase *state.Phase, plan *state.Plan, skipAnalysis bool) *AnalysisResult {
	result := &AnalysisResult{}

	if skipAnalysis {
		return result
	}

	// Read the completed plan to extract discoveries
	planContent, err := os.ReadFile(plan.Path)
	if err != nil {
		result.Error = fmt.Errorf("cannot read plan for analysis: %w", err)
		return result
	}

	discoveries := ParseDiscoveries(string(planContent))
	result.DiscoveriesFound = len(discoveries)

	if len(discoveries) == 0 {
		return result
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("[%s] %s Found %d discoveries, running analysis...\n",
		time.Now().Format("15:04:05"), cyan("Analysis:"), len(discoveries))

	// Find subsequent plans in this phase and future phases
	subsequentPlans := e.findSubsequentPlans(phase, plan)
	if len(subsequentPlans) == 0 {
		fmt.Printf("[%s] %s No subsequent plans to analyze\n",
			time.Now().Format("15:04:05"), yellow("Analysis:"))
		return result
	}

	// Build the analysis prompt
	prompt := e.buildAnalysisPrompt(plan, discoveries, subsequentPlans)

	// Execute analysis with Claude
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			plan.Path,
			filepath.Join(e.config.PlanningDir, "ROADMAP.md"),
		},
		Model: e.config.Model,
		AllowedTools: []string{
			"Read", "Write", "Edit", "Glob", "Grep",
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

	return result
}

// ParseDiscoveries extracts discovery blocks from PLAN.md content
func ParseDiscoveries(content string) []Discovery {
	var discoveries []Discovery

	// Match <discovery> blocks
	pattern := regexp.MustCompile(`(?s)<discovery\s+type="([^"]+)"\s+severity="([^"]+)">\s*` +
		`<title>([^<]+)</title>\s*` +
		`<detail>([^<]+)</detail>\s*` +
		`(?:<file>([^<]*)</file>\s*)?` +
		`<action>([^<]+)</action>\s*` +
		`</discovery>`)

	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 7 {
			d := Discovery{
				Type:     strings.TrimSpace(match[1]),
				Severity: strings.TrimSpace(match[2]),
				Title:    strings.TrimSpace(match[3]),
				Detail:   strings.TrimSpace(match[4]),
				File:     strings.TrimSpace(match[5]),
				Action:   strings.TrimSpace(match[6]),
			}
			discoveries = append(discoveries, d)
		}
	}

	return discoveries
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
func (e *Executor) buildAnalysisPrompt(plan *state.Plan, discoveries []Discovery, subsequentPlans []string) string {
	var discoveriesText strings.Builder
	for i, d := range discoveries {
		discoveriesText.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n", i+1, d.Type, d.Severity, d.Title))
		discoveriesText.WriteString(fmt.Sprintf("   Detail: %s\n", d.Detail))
		if d.File != "" {
			discoveriesText.WriteString(fmt.Sprintf("   File: %s\n", d.File))
		}
		discoveriesText.WriteString(fmt.Sprintf("   Action: %s\n\n", d.Action))
	}

	return fmt.Sprintf(`You are analyzing discoveries from a completed plan execution.

## Just-Completed Plan
%s

## Discoveries from Execution
%s

## Subsequent Plans to Review
%s

## Your Task

Review each discovery and determine its impact on subsequent plans.

For discoveries with action "needs-fix", "needs-implementation", or "needs-plan":
1. Read the relevant subsequent plan files
2. Determine if the discovery:
   - Invalidates tasks in a plan (work is already done, or approach is wrong)
   - Means a dependency must be resolved first
   - Requires a new plan to be created

For each plan that needs updating:
1. Add a note in the plan's <context> section referencing the discovery
2. If a task is invalidated, add a note explaining why
3. If a blocker exists, add a <blocker> tag at the top

## Rules
- Only modify subsequent plans if a discovery directly impacts them
- Do NOT create new plan files - just note what would be needed
- Do NOT modify the completed plan
- Keep changes minimal and targeted

## Completion
When done analyzing, output a brief summary:
- Number of plans reviewed
- Number of plans modified
- Any critical issues that need immediate attention

Signal completion with: ###ANALYSIS_COMPLETE###
`, plan.Path, discoveriesText.String(), strings.Join(subsequentPlans, "\n"))
}

// HasActionableDiscoveries returns true if any discoveries require action
func HasActionableDiscoveries(discoveries []Discovery) bool {
	for _, d := range discoveries {
		switch d.Action {
		case "needs-fix", "needs-implementation", "needs-plan", "needs-investigation":
			return true
		}
	}
	return false
}

// FilterBySeverity returns discoveries at or above the given severity
func FilterBySeverity(discoveries []Discovery, minSeverity string) []Discovery {
	severityOrder := map[string]int{
		"critical": 5,
		"high":     4,
		"medium":   3,
		"low":      2,
		"info":     1,
	}

	minLevel := severityOrder[minSeverity]
	var filtered []Discovery
	for _, d := range discoveries {
		if severityOrder[d.Severity] >= minLevel {
			filtered = append(filtered, d)
		}
	}
	return filtered
}
