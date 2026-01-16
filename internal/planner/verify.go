package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/daydemir/ralph/internal/prompts"
	"github.com/daydemir/ralph/internal/types"
	"github.com/daydemir/ralph/internal/utils"
)

// VerificationIssue represents a single issue found during plan verification
type VerificationIssue struct {
	Plan        string `json:"plan"`
	Dimension   string `json:"dimension"`
	Severity    string `json:"severity"` // blocker, warning, info
	Description string `json:"description"`
	Task        string `json:"task,omitempty"`
	FixHint     string `json:"fix_hint"`
}

// VerificationResult holds the result of plan verification
type VerificationResult struct {
	Status       string              `json:"status"` // passed, issues_found
	PlansChecked int                 `json:"plans_checked"`
	Blockers     int                 `json:"blockers"`
	Warnings     int                 `json:"warnings"`
	Issues       []VerificationIssue `json:"issues"`
}

// PlanVerifier verifies plans before execution
type PlanVerifier struct {
	ClaudeBinary string
	WorkDir      string
	PlanningDir  string
}

// NewPlanVerifier creates a new plan verifier
func NewPlanVerifier(claudeBinary, workDir string) *PlanVerifier {
	if claudeBinary == "" {
		claudeBinary = "claude"
	}
	resolved := utils.ResolveBinaryPath(claudeBinary)
	return &PlanVerifier{
		ClaudeBinary: resolved,
		WorkDir:      workDir,
		PlanningDir:  filepath.Join(workDir, ".planning"),
	}
}

// VerifyPhase verifies all plans in a phase before execution
func (v *PlanVerifier) VerifyPhase(ctx context.Context, phaseNumber int) (*VerificationResult, error) {
	// Find phase directory
	phasesDir := filepath.Join(v.PlanningDir, "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read phases directory: %w", err)
	}

	var phaseDir string
	phasePrefix := fmt.Sprintf("%02d", phaseNumber)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), phasePrefix) {
			phaseDir = filepath.Join(phasesDir, entry.Name())
			break
		}
	}

	if phaseDir == "" {
		return nil, fmt.Errorf("phase %d not found", phaseNumber)
	}

	// Load all plan files
	planEntries, err := os.ReadDir(phaseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read phase directory: %w", err)
	}

	var plans []*types.Plan
	for _, entry := range planEntries {
		if strings.HasSuffix(entry.Name(), ".json") && !strings.Contains(entry.Name(), "SUMMARY") {
			planPath := filepath.Join(phaseDir, entry.Name())
			plan, err := loadPlanJSON(planPath)
			if err != nil {
				continue // Skip malformed plans
			}
			plans = append(plans, plan)
		}
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("no plans found in phase %d", phaseNumber)
	}

	// Perform static verification
	result := v.verifyPlans(plans)

	return result, nil
}

// verifyPlans performs static verification of plan files
func (v *PlanVerifier) verifyPlans(plans []*types.Plan) *VerificationResult {
	result := &VerificationResult{
		Status:       "passed",
		PlansChecked: len(plans),
		Issues:       []VerificationIssue{},
	}

	for _, plan := range plans {
		// Dimension 1: Task Completeness
		v.checkTaskCompleteness(plan, result)

		// Dimension 2: Scope Sanity
		v.checkScopeSanity(plan, result)

		// Dimension 3: Verification Presence
		v.checkVerificationPresence(plan, result)
	}

	// Count blockers and warnings
	for _, issue := range result.Issues {
		switch issue.Severity {
		case "blocker":
			result.Blockers++
		case "warning":
			result.Warnings++
		}
	}

	if result.Blockers > 0 {
		result.Status = "issues_found"
	}

	return result
}

// checkTaskCompleteness verifies all tasks have required fields
func (v *PlanVerifier) checkTaskCompleteness(plan *types.Plan, result *VerificationResult) {
	for _, task := range plan.Tasks {
		if task.Type == types.TaskTypeAuto {
			// Auto tasks need: files, action, verify, done
			if len(task.Files) == 0 {
				result.Issues = append(result.Issues, VerificationIssue{
					Plan:        plan.PlanNumber,
					Dimension:   "task_completeness",
					Severity:    "warning",
					Description: fmt.Sprintf("Task '%s' has empty files array", task.Name),
					Task:        task.ID,
					FixHint:     "Specify which files this task creates or modifies",
				})
			}

			if task.Action == "" {
				result.Issues = append(result.Issues, VerificationIssue{
					Plan:        plan.PlanNumber,
					Dimension:   "task_completeness",
					Severity:    "blocker",
					Description: fmt.Sprintf("Task '%s' missing action field", task.Name),
					Task:        task.ID,
					FixHint:     "Add specific implementation instructions",
				})
			} else if len(task.Action) < 20 {
				result.Issues = append(result.Issues, VerificationIssue{
					Plan:        plan.PlanNumber,
					Dimension:   "task_completeness",
					Severity:    "warning",
					Description: fmt.Sprintf("Task '%s' has very short action - may be too vague", task.Name),
					Task:        task.ID,
					FixHint:     "Add more specific implementation details",
				})
			}

			if task.Verify == "" {
				result.Issues = append(result.Issues, VerificationIssue{
					Plan:        plan.PlanNumber,
					Dimension:   "task_completeness",
					Severity:    "blocker",
					Description: fmt.Sprintf("Task '%s' missing verify field", task.Name),
					Task:        task.ID,
					FixHint:     "Add command or check to verify task completion",
				})
			}

			if task.Done == "" {
				result.Issues = append(result.Issues, VerificationIssue{
					Plan:        plan.PlanNumber,
					Dimension:   "task_completeness",
					Severity:    "warning",
					Description: fmt.Sprintf("Task '%s' missing done criteria", task.Name),
					Task:        task.ID,
					FixHint:     "Add measurable acceptance criteria",
				})
			}
		}
	}
}

// checkScopeSanity verifies plans fit within context budget
func (v *PlanVerifier) checkScopeSanity(plan *types.Plan, result *VerificationResult) {
	taskCount := len(plan.Tasks)

	if taskCount > 5 {
		result.Issues = append(result.Issues, VerificationIssue{
			Plan:        plan.PlanNumber,
			Dimension:   "scope_sanity",
			Severity:    "blocker",
			Description: fmt.Sprintf("Plan has %d tasks - exceeds maximum of 5", taskCount),
			FixHint:     "Split into multiple smaller plans with 2-3 tasks each",
		})
	} else if taskCount > 3 {
		result.Issues = append(result.Issues, VerificationIssue{
			Plan:        plan.PlanNumber,
			Dimension:   "scope_sanity",
			Severity:    "warning",
			Description: fmt.Sprintf("Plan has %d tasks - consider splitting for better quality", taskCount),
			FixHint:     "Consider splitting into multiple plans with 2-3 tasks each",
		})
	}

	// Count total files
	totalFiles := 0
	for _, task := range plan.Tasks {
		totalFiles += len(task.Files)
	}

	if totalFiles > 15 {
		result.Issues = append(result.Issues, VerificationIssue{
			Plan:        plan.PlanNumber,
			Dimension:   "scope_sanity",
			Severity:    "blocker",
			Description: fmt.Sprintf("Plan modifies %d files - exceeds maximum of 15", totalFiles),
			FixHint:     "Split into multiple plans touching fewer files each",
		})
	} else if totalFiles > 10 {
		result.Issues = append(result.Issues, VerificationIssue{
			Plan:        plan.PlanNumber,
			Dimension:   "scope_sanity",
			Severity:    "warning",
			Description: fmt.Sprintf("Plan modifies %d files - approaching context limit", totalFiles),
			FixHint:     "Consider splitting to stay well within context budget",
		})
	}
}

// checkVerificationPresence ensures plans have verification criteria
func (v *PlanVerifier) checkVerificationPresence(plan *types.Plan, result *VerificationResult) {
	if len(plan.Verification) == 0 {
		result.Issues = append(result.Issues, VerificationIssue{
			Plan:        plan.PlanNumber,
			Dimension:   "verification_presence",
			Severity:    "warning",
			Description: "Plan has no verification criteria",
			FixHint:     "Add verification commands like 'npm run build', 'npm test'",
		})
	}
}

// VerifyWithAgent runs the plan-checker agent for deeper verification
func (v *PlanVerifier) VerifyWithAgent(ctx context.Context, phaseNumber int) (*VerificationResult, error) {
	// Load the plan-checker agent prompt
	prompt, err := prompts.GetAgent("plan-checker")
	if err != nil {
		return nil, fmt.Errorf("cannot load plan-checker agent: %w", err)
	}

	// Build context for the agent
	contextPrompt := fmt.Sprintf(`%s

## Task

Verify all plans in phase %d. Check all six dimensions:
1. Requirement coverage
2. Task completeness
3. Dependency correctness
4. Key links planned
5. Scope sanity
6. Verification derivation

Return structured JSON with issues found.

## Phase to Verify

Phase: %d

Begin verification.`, prompt, phaseNumber, phaseNumber)

	// Execute with Claude
	args := []string{
		"--print",
		"-p", contextPrompt,
	}

	cmd := exec.CommandContext(ctx, v.ClaudeBinary, args...)
	cmd.Dir = v.WorkDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("verification agent failed: %w", err)
	}

	// Try to parse structured result from output
	result := &VerificationResult{
		Status:       "passed",
		PlansChecked: 0,
		Issues:       []VerificationIssue{},
	}

	// Look for JSON in the output
	outputStr := string(output)
	if idx := strings.Index(outputStr, `{"status":`); idx >= 0 {
		endIdx := strings.LastIndex(outputStr, "}")
		if endIdx > idx {
			jsonStr := outputStr[idx : endIdx+1]
			if err := json.Unmarshal([]byte(jsonStr), result); err == nil {
				return result, nil
			}
		}
	}

	// If we can't parse JSON, fall back to static verification
	return v.VerifyPhase(ctx, phaseNumber)
}

// loadPlanJSON loads a plan from a JSON file
func loadPlanJSON(path string) (*types.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var plan types.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

// FormatVerificationResult formats the result for display
func FormatVerificationResult(result *VerificationResult) string {
	var sb strings.Builder

	if result.Status == "passed" {
		sb.WriteString("## VERIFICATION PASSED\n\n")
		sb.WriteString(fmt.Sprintf("**Plans checked:** %d\n", result.PlansChecked))
		sb.WriteString("\nAll checks passed. Ready for execution.\n")
	} else {
		sb.WriteString("## ISSUES FOUND\n\n")
		sb.WriteString(fmt.Sprintf("**Plans checked:** %d\n", result.PlansChecked))
		sb.WriteString(fmt.Sprintf("**Blockers:** %d\n", result.Blockers))
		sb.WriteString(fmt.Sprintf("**Warnings:** %d\n\n", result.Warnings))

		if result.Blockers > 0 {
			sb.WriteString("### Blockers (must fix)\n\n")
			for i, issue := range result.Issues {
				if issue.Severity == "blocker" {
					sb.WriteString(fmt.Sprintf("**%d. [%s] %s**\n", i+1, issue.Dimension, issue.Description))
					sb.WriteString(fmt.Sprintf("- Plan: %s\n", issue.Plan))
					if issue.Task != "" {
						sb.WriteString(fmt.Sprintf("- Task: %s\n", issue.Task))
					}
					sb.WriteString(fmt.Sprintf("- Fix: %s\n\n", issue.FixHint))
				}
			}
		}

		if result.Warnings > 0 {
			sb.WriteString("### Warnings (should fix)\n\n")
			for i, issue := range result.Issues {
				if issue.Severity == "warning" {
					sb.WriteString(fmt.Sprintf("**%d. [%s] %s**\n", i+1, issue.Dimension, issue.Description))
					sb.WriteString(fmt.Sprintf("- Plan: %s\n", issue.Plan))
					if issue.Task != "" {
						sb.WriteString(fmt.Sprintf("- Task: %s\n", issue.Task))
					}
					sb.WriteString(fmt.Sprintf("- Fix: %s\n\n", issue.FixHint))
				}
			}
		}
	}

	return sb.String()
}
