package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "uppercase to lowercase",
			input:    "HELLO",
			expected: "hello",
		},
		{
			name:     "mixed case",
			input:    "Hello World",
			expected: "hello-world",
		},
		{
			name:     "spaces to hyphens",
			input:    "critical bug fixes",
			expected: "critical-bug-fixes",
		},
		{
			name:     "multiple spaces",
			input:    "hello   world",
			expected: "hello---world",
		},
		{
			name:     "special characters removed",
			input:    "Hello! World?",
			expected: "hello-world",
		},
		{
			name:     "numbers preserved",
			input:    "Phase 1 Setup",
			expected: "phase-1-setup",
		},
		{
			name:     "underscores removed",
			input:    "hello_world",
			expected: "helloworld",
		},
		{
			name:     "hyphens preserved",
			input:    "hello-world",
			expected: "hello-world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    "!@#$%",
			expected: "",
		},
		{
			name:     "typical phase name",
			input:    "Critical Bug Fixes",
			expected: "critical-bug-fixes",
		},
		{
			name:     "apostrophe removed",
			input:    "User's Guide",
			expected: "users-guide",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoadPlanJSON(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("load valid JSON", func(t *testing.T) {
		validJSON := `{
  "phase": "01-critical-fixes",
  "plan_number": "01",
  "status": "pending",
  "objective": "Fix critical bugs",
  "tasks": [
    {
      "id": "task-1",
      "name": "Fix bug",
      "type": "auto",
      "files": ["main.go"],
      "action": "Fix the bug",
      "verify": "run tests",
      "done": "tests pass",
      "status": "pending"
    }
  ],
  "verification": ["go test ./..."],
  "created_at": "2024-01-15T10:00:00Z"
}`
		planPath := filepath.Join(tempDir, "valid-plan.json")
		if err := os.WriteFile(planPath, []byte(validJSON), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		plan, err := LoadPlanJSON(planPath)
		if err != nil {
			t.Fatalf("LoadPlanJSON() unexpected error = %v", err)
		}

		if plan.Phase != "01-critical-fixes" {
			t.Errorf("plan.Phase = %q, want %q", plan.Phase, "01-critical-fixes")
		}
		if plan.PlanNumber != "01" {
			t.Errorf("plan.PlanNumber = %q, want %q", plan.PlanNumber, "01")
		}
		if plan.Status != types.StatusPending {
			t.Errorf("plan.Status = %q, want %q", plan.Status, types.StatusPending)
		}
		if plan.Objective != "Fix critical bugs" {
			t.Errorf("plan.Objective = %q, want %q", plan.Objective, "Fix critical bugs")
		}
		if len(plan.Tasks) != 1 {
			t.Errorf("len(plan.Tasks) = %d, want 1", len(plan.Tasks))
		}
	})

	t.Run("load invalid JSON syntax", func(t *testing.T) {
		invalidJSON := `{
  "phase": "01-critical-fixes",
  "plan_number": "01",
  invalid json here
}`
		planPath := filepath.Join(tempDir, "invalid-syntax.json")
		if err := os.WriteFile(planPath, []byte(invalidJSON), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := LoadPlanJSON(planPath)
		if err == nil {
			t.Error("LoadPlanJSON() expected error for invalid JSON syntax, got nil")
		}
	})

	t.Run("load JSON with unknown fields", func(t *testing.T) {
		jsonWithUnknown := `{
  "phase": "01-critical-fixes",
  "plan_number": "01",
  "status": "pending",
  "objective": "Fix critical bugs",
  "unknown_field": "should fail",
  "tasks": [
    {
      "id": "task-1",
      "name": "Fix bug",
      "type": "auto",
      "action": "Fix the bug",
      "status": "pending"
    }
  ],
  "verification": [],
  "created_at": "2024-01-15T10:00:00Z"
}`
		planPath := filepath.Join(tempDir, "unknown-fields.json")
		if err := os.WriteFile(planPath, []byte(jsonWithUnknown), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := LoadPlanJSON(planPath)
		if err == nil {
			t.Error("LoadPlanJSON() expected error for unknown fields, got nil")
		}
	})

	t.Run("load non-existent file", func(t *testing.T) {
		_, err := LoadPlanJSON(filepath.Join(tempDir, "non-existent.json"))
		if err == nil {
			t.Error("LoadPlanJSON() expected error for non-existent file, got nil")
		}
	})

	t.Run("load JSON missing required fields", func(t *testing.T) {
		// Missing phase field
		missingPhaseJSON := `{
  "plan_number": "01",
  "status": "pending",
  "objective": "Fix critical bugs",
  "tasks": [
    {
      "id": "task-1",
      "name": "Fix bug",
      "type": "auto",
      "action": "Fix the bug",
      "status": "pending"
    }
  ],
  "verification": [],
  "created_at": "2024-01-15T10:00:00Z"
}`
		planPath := filepath.Join(tempDir, "missing-phase.json")
		if err := os.WriteFile(planPath, []byte(missingPhaseJSON), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := LoadPlanJSON(planPath)
		if err == nil {
			t.Error("LoadPlanJSON() expected error for missing phase, got nil")
		}
	})

	t.Run("load JSON with empty tasks", func(t *testing.T) {
		emptyTasksJSON := `{
  "phase": "01-critical-fixes",
  "plan_number": "01",
  "status": "pending",
  "objective": "Fix critical bugs",
  "tasks": [],
  "verification": [],
  "created_at": "2024-01-15T10:00:00Z"
}`
		planPath := filepath.Join(tempDir, "empty-tasks.json")
		if err := os.WriteFile(planPath, []byte(emptyTasksJSON), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := LoadPlanJSON(planPath)
		if err == nil {
			t.Error("LoadPlanJSON() expected error for empty tasks, got nil")
		}
	})
}

func TestSavePlanJSON(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "ralph-test-save-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("save valid plan", func(t *testing.T) {
		plan := &types.Plan{
			Phase:      "01-critical-fixes",
			PlanNumber: "01",
			Status:     types.StatusPending,
			Objective:  "Fix critical bugs",
			Tasks: []types.Task{
				{
					ID:     "task-1",
					Name:   "Fix bug",
					Type:   types.TaskTypeAuto,
					Action: "Fix the bug",
					Verify: "run tests",
					Done:   "tests pass",
					Status: types.StatusPending,
				},
			},
			Verification: []string{"go test ./..."},
			CreatedAt:    time.Now(),
		}

		planPath := filepath.Join(tempDir, "saved-plan.json")
		err := SavePlanJSON(planPath, plan)
		if err != nil {
			t.Fatalf("SavePlanJSON() unexpected error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(planPath); os.IsNotExist(err) {
			t.Error("SavePlanJSON() did not create file")
		}

		// Verify we can load it back
		loadedPlan, err := LoadPlanJSON(planPath)
		if err != nil {
			t.Fatalf("LoadPlanJSON() failed to load saved plan: %v", err)
		}

		if loadedPlan.Phase != plan.Phase {
			t.Errorf("loaded plan.Phase = %q, want %q", loadedPlan.Phase, plan.Phase)
		}
	})

	t.Run("save invalid plan fails", func(t *testing.T) {
		invalidPlan := &types.Plan{
			Phase:      "", // Missing required field
			PlanNumber: "01",
			Status:     types.StatusPending,
			Objective:  "Fix critical bugs",
			Tasks: []types.Task{
				{
					ID:     "task-1",
					Name:   "Fix bug",
					Type:   types.TaskTypeAuto,
					Action: "Fix the bug",
					Verify: "run tests",
					Done:   "tests pass",
					Status: types.StatusPending,
				},
			},
			CreatedAt: time.Now(),
		}

		planPath := filepath.Join(tempDir, "invalid-plan.json")
		err := SavePlanJSON(planPath, invalidPlan)
		if err == nil {
			t.Error("SavePlanJSON() expected error for invalid plan, got nil")
		}
	})
}
