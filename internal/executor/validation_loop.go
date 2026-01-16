package executor

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/state"
	"github.com/daydemir/ralph/internal/types"
)

// HealOptions contains options for the validation and healing process
type HealOptions struct {
	// MaxRetries sets an optional retry limit (0 = no limit, use iteration count)
	// Only set if user explicitly specifies - default to unlimited retries
	MaxRetries int

	// ClaudeBinary is the path to the claude executable
	ClaudeBinary string

	// WorkDir is the working directory for Claude execution
	WorkDir string
}

// ValidateAndHeal loads a plan JSON, validates it, and if validation fails,
// calls Claude to fix the errors. This continues until validation passes or
// the iteration limit is reached (no arbitrary retry cap - autonomy principle).
func ValidateAndHeal(ctx context.Context, planPath string, opts *HealOptions) (*types.Plan, error) {
	if opts == nil {
		opts = &HealOptions{}
	}

	// Set working directory to plan's directory if not specified
	if opts.WorkDir == "" {
		opts.WorkDir = filepath.Dir(planPath)
	}

	// Track iteration count (no arbitrary cap unless user specified)
	iteration := 0

	for {
		// Attempt to load and validate the plan
		plan, err := state.LoadPlanJSON(planPath)
		if err == nil {
			// Validation passed!
			return plan, nil
		}

		// Check if we've hit the retry limit (if specified)
		if opts.MaxRetries > 0 && iteration >= opts.MaxRetries {
			return nil, fmt.Errorf("validation failed after %d retries: %w", iteration, err)
		}

		// Extract validation errors for Claude
		validationErr, ok := err.(interface{ ToPrompt() string })
		if !ok {
			// Not a structured validation error - can't heal automatically
			return nil, fmt.Errorf("cannot auto-heal: %w", err)
		}

		// Generate schema documentation from Go types
		schema := generateSchemaPrompt()

		// Call Claude to fix the validation errors
		if err := callClaudeToFix(ctx, planPath, validationErr.ToPrompt(), schema, opts); err != nil {
			return nil, fmt.Errorf("claude fix failed: %w", err)
		}

		iteration++
	}
}

// generateSchemaPrompt returns the Go type definitions as schema documentation
// This is the "single source of truth" - Go types ARE the documentation
func generateSchemaPrompt() string {
	var sb strings.Builder

	sb.WriteString("# Ralph Plan JSON Schema\n\n")
	sb.WriteString("The plan JSON must conform to the following Go type definitions:\n\n")

	sb.WriteString("## Plan Structure\n\n")
	sb.WriteString("```go\n")
	sb.WriteString("type Plan struct {\n")
	sb.WriteString("    Phase        string     // Phase ID like \"01-critical-bug-fixes\"\n")
	sb.WriteString("    PlanNumber   string     // Plan number like \"01\" or \"01.1\"\n")
	sb.WriteString("    Status       Status     // One of: pending, in_progress, complete, failed\n")
	sb.WriteString("    Objective    string     // Plan objective (required)\n")
	sb.WriteString("    Tasks        []Task     // List of tasks (at least 1 required)\n")
	sb.WriteString("    Verification []string   // Verification commands\n")
	sb.WriteString("    CreatedAt    time.Time  // ISO 8601 timestamp\n")
	sb.WriteString("    CompletedAt  *time.Time // Optional completion timestamp\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Task Structure\n\n")
	sb.WriteString("```go\n")
	sb.WriteString("type Task struct {\n")
	sb.WriteString("    ID          string     // Task ID like \"task-1\"\n")
	sb.WriteString("    Name        string     // Task name (required)\n")
	sb.WriteString("    Type        TaskType   // One of: auto, manual\n")
	sb.WriteString("    Files       []string   // Files to create/modify\n")
	sb.WriteString("    Action      string     // What to do (required)\n")
	sb.WriteString("    Verify      string     // How to verify\n")
	sb.WriteString("    Done        string     // Acceptance criteria\n")
	sb.WriteString("    Status      Status     // One of: pending, in_progress, complete, failed\n")
	sb.WriteString("    CompletedAt *time.Time // Optional completion timestamp\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Valid Enum Values\n\n")
	sb.WriteString("**TaskType:**\n")
	sb.WriteString("- `auto` - Executed fully autonomously by Claude\n")
	sb.WriteString("- `manual` - Requires human interaction\n\n")
	sb.WriteString("**INVALID:** checkpoint:human-verify, checkpoint:decision, checkpoint:human-action are NOT valid task types.\n\n")

	sb.WriteString("**Status:**\n")
	sb.WriteString("- `pending` - Work has not started\n")
	sb.WriteString("- `in_progress` - Work is currently executing\n")
	sb.WriteString("- `complete` - Work has successfully finished\n")
	sb.WriteString("- `failed` - Work has failed\n\n")

	sb.WriteString("## Common Validation Errors\n\n")
	sb.WriteString("1. **Invalid task type:** Must be \"auto\" or \"manual\" (not checkpoint types)\n")
	sb.WriteString("2. **Missing required fields:** phase, plan_number, objective, tasks, created_at\n")
	sb.WriteString("3. **Empty tasks array:** At least one task is required\n")
	sb.WriteString("4. **Invalid status:** Must be one of the valid Status enum values\n")
	sb.WriteString("5. **Unknown fields:** JSON decoder uses DisallowUnknownFields - remove any extra fields\n")

	return sb.String()
}

// callClaudeToFix executes Claude to fix validation errors in the plan JSON
func callClaudeToFix(ctx context.Context, planPath string, validationErrors string, schema string, opts *HealOptions) error {
	// Build the fix prompt
	prompt := buildFixPrompt(planPath, validationErrors, schema)

	// Create Claude instance
	claude := llm.NewClaude(opts.ClaudeBinary)

	// Execute Claude to fix the file
	executeOpts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			planPath, // Claude will read and edit this file
		},
		WorkDir:      opts.WorkDir,
		AllowedTools: []string{"Read", "Edit", "Write"}, // Only allow file editing
	}

	// Execute Claude and capture output
	reader, err := claude.Execute(ctx, executeOpts)
	if err != nil {
		return fmt.Errorf("failed to execute claude: %w", err)
	}
	defer reader.Close()

	// Read output to ensure Claude completed
	// In a full implementation, we'd parse the stream-json output
	// For now, we just consume the output and let it complete
	_, readErr := io.ReadAll(reader)
	if readErr != nil {
		return fmt.Errorf("failed to read claude output: %w", readErr)
	}

	return nil
}

// buildFixPrompt constructs the prompt for Claude to fix validation errors
func buildFixPrompt(planPath, validationErrors, schema string) string {
	var sb strings.Builder

	sb.WriteString("Fix the following validation errors in ")
	sb.WriteString(filepath.Base(planPath))
	sb.WriteString(":\n\n")
	sb.WriteString(validationErrors)
	sb.WriteString("\n\n")
	sb.WriteString("## Expected Schema\n\n")
	sb.WriteString(schema)
	sb.WriteString("\n\n")
	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Read the current JSON file\n")
	sb.WriteString("2. Fix ALL validation errors listed above\n")
	sb.WriteString("3. Ensure the JSON conforms to the schema\n")
	sb.WriteString("4. Save the corrected JSON back to the file\n")
	sb.WriteString("\n")
	sb.WriteString("Use the Edit tool to fix the errors precisely. Do not change anything except what's needed to fix the validation errors.\n")

	return sb.String()
}
