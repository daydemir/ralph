package planner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/daydemir/ralph/internal/prompts"
	"github.com/daydemir/ralph/internal/utils"
)

// DiscoveryLevel represents the depth of research needed
type DiscoveryLevel int

const (
	DiscoveryLevelSkip     DiscoveryLevel = 0 // Pure internal work
	DiscoveryLevelQuick    DiscoveryLevel = 1 // 2-5 min verification
	DiscoveryLevelStandard DiscoveryLevel = 2 // 15-30 min research
	DiscoveryLevelDeep     DiscoveryLevel = 3 // 1+ hour deep dive
)

// PhaseResearcher handles phase research before planning
type PhaseResearcher struct {
	ClaudeBinary string
	WorkDir      string
	PlanningDir  string
}

// ResearchResult holds the result of phase research
type ResearchResult struct {
	PhaseNumber    int            `json:"phase_number"`
	PhaseName      string         `json:"phase_name"`
	DiscoveryLevel DiscoveryLevel `json:"discovery_level"`
	Summary        string         `json:"summary"`
	Recommendation string         `json:"recommendation"`
	KeyFindings    []string       `json:"key_findings"`
	Risks          []string       `json:"risks"`
	OutputPath     string         `json:"output_path,omitempty"` // Path to RESEARCH.md if created
}

// NewPhaseResearcher creates a new phase researcher
func NewPhaseResearcher(claudeBinary, workDir string) *PhaseResearcher {
	if claudeBinary == "" {
		claudeBinary = "claude"
	}
	resolved := utils.ResolveBinaryPath(claudeBinary)
	return &PhaseResearcher{
		ClaudeBinary: resolved,
		WorkDir:      workDir,
		PlanningDir:  filepath.Join(workDir, ".planning"),
	}
}

// DetermineDiscoveryLevel analyzes a phase description to determine research depth
func (r *PhaseResearcher) DetermineDiscoveryLevel(phaseDescription string) DiscoveryLevel {
	desc := strings.ToLower(phaseDescription)

	// Level 3 indicators
	level3Keywords := []string{
		"architecture", "design pattern", "system design",
		"authentication system", "authorization",
		"data model", "database schema",
		"multiple services", "microservices",
		"machine learning", "ml model",
		"3d", "webgl", "shader",
		"real-time", "websocket",
	}
	for _, kw := range level3Keywords {
		if strings.Contains(desc, kw) {
			return DiscoveryLevelDeep
		}
	}

	// Level 2 indicators
	level2Keywords := []string{
		"choose", "select", "evaluate", "compare",
		"new api", "external service", "third-party",
		"integrate", "integration",
		"payment", "stripe", "billing",
		"email", "sendgrid", "ses",
		"storage", "s3", "cloudinary",
	}
	for _, kw := range level2Keywords {
		if strings.Contains(desc, kw) {
			return DiscoveryLevelStandard
		}
	}

	// Level 1 indicators
	level1Keywords := []string{
		"update", "modify", "add field",
		"use existing", "follow pattern",
		"simple", "basic", "crud",
	}
	for _, kw := range level1Keywords {
		if strings.Contains(desc, kw) {
			return DiscoveryLevelQuick
		}
	}

	// Default to skip for internal work
	internalKeywords := []string{
		"refactor", "cleanup", "fix bug",
		"add button", "update ui", "style",
		"internal", "existing",
	}
	for _, kw := range internalKeywords {
		if strings.Contains(desc, kw) {
			return DiscoveryLevelSkip
		}
	}

	// Default to quick for uncertain cases
	return DiscoveryLevelQuick
}

// ResearchPhase conducts research for a phase
func (r *PhaseResearcher) ResearchPhase(ctx context.Context, phaseNumber int, phaseDescription string) (*ResearchResult, error) {
	level := r.DetermineDiscoveryLevel(phaseDescription)

	result := &ResearchResult{
		PhaseNumber:    phaseNumber,
		DiscoveryLevel: level,
	}

	// Level 0: No research needed
	if level == DiscoveryLevelSkip {
		result.Summary = "No research needed - pure internal work following existing patterns"
		result.Recommendation = "Proceed directly to planning"
		return result, nil
	}

	// Level 1: Quick verification (no agent needed, just notes)
	if level == DiscoveryLevelQuick {
		return r.quickResearch(ctx, phaseNumber, phaseDescription)
	}

	// Level 2-3: Use researcher agent
	return r.agentResearch(ctx, phaseNumber, phaseDescription, level)
}

// quickResearch performs quick verification without full agent
func (r *PhaseResearcher) quickResearch(ctx context.Context, phaseNumber int, description string) (*ResearchResult, error) {
	result := &ResearchResult{
		PhaseNumber:    phaseNumber,
		DiscoveryLevel: DiscoveryLevelQuick,
		Summary:        "Quick verification of approach",
		KeyFindings:    []string{},
	}

	// For quick research, we just verify we have the tools/patterns
	// This is a lightweight check, not a full research session

	// Check if any common patterns apply
	desc := strings.ToLower(description)

	if strings.Contains(desc, "auth") {
		result.KeyFindings = append(result.KeyFindings,
			"Authentication: Use jose for JWT, bcrypt for passwords")
		result.Recommendation = "Follow existing auth patterns in codebase"
	}

	if strings.Contains(desc, "api") || strings.Contains(desc, "endpoint") {
		result.KeyFindings = append(result.KeyFindings,
			"API: Follow existing route handler patterns")
		result.Recommendation = "Use established API patterns"
	}

	if strings.Contains(desc, "ui") || strings.Contains(desc, "component") {
		result.KeyFindings = append(result.KeyFindings,
			"UI: Follow existing component patterns")
		result.Recommendation = "Use established component patterns"
	}

	if len(result.KeyFindings) == 0 {
		result.KeyFindings = append(result.KeyFindings,
			"No specific patterns identified - follow existing codebase conventions")
		result.Recommendation = "Follow existing codebase patterns"
	}

	return result, nil
}

// agentResearch uses the researcher agent for deeper analysis
func (r *PhaseResearcher) agentResearch(ctx context.Context, phaseNumber int, description string, level DiscoveryLevel) (*ResearchResult, error) {
	// Load the researcher agent prompt
	prompt, err := prompts.GetAgent("researcher")
	if err != nil {
		return nil, fmt.Errorf("cannot load researcher agent: %w", err)
	}

	levelName := "Standard"
	if level == DiscoveryLevelDeep {
		levelName = "Deep"
	}

	// Build context for the agent
	contextPrompt := fmt.Sprintf(`%s

## Research Task

**Phase:** %d
**Description:** %s
**Level:** %s (Level %d)

Conduct research at the specified level. Evaluate options, identify best practices, and provide a recommendation.

If Level 3, create a comprehensive RESEARCH.md document.
If Level 2, provide a detailed summary.

Begin research.`, prompt, phaseNumber, description, levelName, level)

	// Execute with Claude
	args := []string{
		"--print",
		"-p", contextPrompt,
	}

	cmd := exec.CommandContext(ctx, r.ClaudeBinary, args...)
	cmd.Dir = r.WorkDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("research agent failed: %w", err)
	}

	// Parse result from output
	result := &ResearchResult{
		PhaseNumber:    phaseNumber,
		DiscoveryLevel: level,
		Summary:        extractSection(string(output), "Summary", "Recommendation"),
		Recommendation: extractSection(string(output), "Recommendation", "Key Findings"),
		KeyFindings:    extractList(string(output), "Key Findings"),
		Risks:          extractList(string(output), "Risks"),
	}

	// Check if RESEARCH.md was created
	phaseDir, err := r.findPhaseDir(phaseNumber)
	if err == nil {
		researchPath := filepath.Join(phaseDir, fmt.Sprintf("%02d-RESEARCH.md", phaseNumber))
		if _, err := os.Stat(researchPath); err == nil {
			result.OutputPath = researchPath
		}
	}

	return result, nil
}

// findPhaseDir finds the phase directory for a given phase number
func (r *PhaseResearcher) findPhaseDir(phaseNumber int) (string, error) {
	phasesDir := filepath.Join(r.PlanningDir, "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return "", err
	}

	prefix := fmt.Sprintf("%02d", phaseNumber)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			return filepath.Join(phasesDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("phase %d not found", phaseNumber)
}

// extractSection extracts a section from markdown output
func extractSection(output, sectionName, nextSection string) string {
	lines := strings.Split(output, "\n")
	var result []string
	inSection := false

	for _, line := range lines {
		if strings.Contains(line, sectionName) {
			inSection = true
			continue
		}
		if inSection && nextSection != "" && strings.Contains(line, nextSection) {
			break
		}
		if inSection {
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// extractList extracts a bulleted list from markdown output
func extractList(output, sectionName string) []string {
	section := extractSection(output, sectionName, "")
	lines := strings.Split(section, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			if item != "" {
				result = append(result, item)
			}
		} else if strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "3.") {
			// Numbered list
			parts := strings.SplitN(line, ".", 2)
			if len(parts) == 2 {
				item := strings.TrimSpace(parts[1])
				if item != "" {
					result = append(result, item)
				}
			}
		}
	}

	return result
}

// FormatResearchResult formats the research result for display
func FormatResearchResult(result *ResearchResult) string {
	var sb strings.Builder

	sb.WriteString("## RESEARCH COMPLETE\n\n")
	sb.WriteString(fmt.Sprintf("**Phase:** %d\n", result.PhaseNumber))
	sb.WriteString(fmt.Sprintf("**Level:** %d (%s)\n\n", result.DiscoveryLevel, levelName(result.DiscoveryLevel)))

	if result.Summary != "" {
		sb.WriteString("### Summary\n")
		sb.WriteString(result.Summary)
		sb.WriteString("\n\n")
	}

	if result.Recommendation != "" {
		sb.WriteString("### Recommendation\n")
		sb.WriteString(result.Recommendation)
		sb.WriteString("\n\n")
	}

	if len(result.KeyFindings) > 0 {
		sb.WriteString("### Key Findings\n")
		for i, finding := range result.KeyFindings {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, finding))
		}
		sb.WriteString("\n")
	}

	if len(result.Risks) > 0 {
		sb.WriteString("### Risks\n")
		for _, risk := range result.Risks {
			sb.WriteString(fmt.Sprintf("- %s\n", risk))
		}
		sb.WriteString("\n")
	}

	if result.OutputPath != "" {
		sb.WriteString(fmt.Sprintf("**Research document:** %s\n\n", result.OutputPath))
	}

	sb.WriteString("Ready for planning.\n")

	return sb.String()
}

func levelName(level DiscoveryLevel) string {
	switch level {
	case DiscoveryLevelSkip:
		return "Skip"
	case DiscoveryLevelQuick:
		return "Quick"
	case DiscoveryLevelStandard:
		return "Standard"
	case DiscoveryLevelDeep:
		return "Deep"
	default:
		return "Unknown"
	}
}
