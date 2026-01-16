package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/daydemir/ralph/internal/types"
	"github.com/daydemir/ralph/internal/utils"
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
			got := utils.Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
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

// TestFindNextPlanJSON_RoadmapSourceOfTruth is a regression test for GitHub Issue #14
// Ensures that phase enumeration comes from roadmap.json, not filesystem scanning
// Stale phase directories should be ignored if not in roadmap
func TestFindNextPlanJSON_RoadmapSourceOfTruth(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "ralph-test-roadmap-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	planningDir := tempDir
	phasesDir := filepath.Join(planningDir, "phases")
	os.MkdirAll(phasesDir, 0755)

	// Create roadmap.json with only phases 1 and 3 (phase 2 intentionally missing)
	roadmap := types.Roadmap{
		Version:     "1.0",
		ProjectName: "Test Project",
		Overview:    "Test",
		Phases: []types.Phase{
			{Number: 1, Name: "Phase One", Goal: "First phase", Status: types.StatusComplete},
			{Number: 3, Name: "Phase Three", Goal: "Third phase", Status: types.StatusPending},
		},
	}
	roadmapData, _ := json.MarshalIndent(roadmap, "", "  ")
	os.WriteFile(filepath.Join(planningDir, "roadmap.json"), roadmapData, 0644)

	// Create phase directories - including a STALE phase 2 that's not in roadmap
	phase1Dir := filepath.Join(phasesDir, "01-phase-one")
	phase2Dir := filepath.Join(phasesDir, "02-stale-phase") // Not in roadmap!
	phase3Dir := filepath.Join(phasesDir, "03-phase-three")
	os.MkdirAll(phase1Dir, 0755)
	os.MkdirAll(phase2Dir, 0755)
	os.MkdirAll(phase3Dir, 0755)

	// Create completed plan in phase 1
	plan1 := &types.Plan{
		Phase:      "01-phase-one",
		PlanNumber: "01",
		Status:     types.StatusComplete,
		Objective:  "Phase 1 Plan",
		Tasks: []types.Task{{
			ID: "task-1", Name: "Task 1", Type: types.TaskTypeAuto,
			Action: "Do task", Verify: "verify", Done: "done", Status: types.StatusComplete,
		}},
		Verification: []string{},
		CreatedAt:    time.Now(),
	}
	SavePlanJSON(filepath.Join(phase1Dir, "01-01.json"), plan1)

	// Create a plan in the STALE phase 2 (should be IGNORED)
	plan2Stale := &types.Plan{
		Phase:      "02-stale-phase",
		PlanNumber: "01",
		Status:     types.StatusPending, // This is pending but should be ignored!
		Objective:  "Stale Phase Plan",
		Tasks: []types.Task{{
			ID: "task-1", Name: "Task 1", Type: types.TaskTypeAuto,
			Action: "Do task", Verify: "verify", Done: "done", Status: types.StatusPending,
		}},
		Verification: []string{},
		CreatedAt:    time.Now(),
	}
	SavePlanJSON(filepath.Join(phase2Dir, "02-01.json"), plan2Stale)

	// Create pending plan in phase 3
	plan3 := &types.Plan{
		Phase:      "03-phase-three",
		PlanNumber: "01",
		Status:     types.StatusPending,
		Objective:  "Phase 3 Plan",
		Tasks: []types.Task{{
			ID: "task-1", Name: "Task 1", Type: types.TaskTypeAuto,
			Action: "Do task", Verify: "verify", Done: "done", Status: types.StatusPending,
		}},
		Verification: []string{},
		CreatedAt:    time.Now(),
	}
	SavePlanJSON(filepath.Join(phase3Dir, "03-01.json"), plan3)

	// Find next plan - should skip stale phase 2 and find phase 3
	nextPhase, nextPlan, err := FindNextPlanJSON(planningDir)
	if err != nil {
		t.Fatalf("FindNextPlanJSON() unexpected error = %v", err)
	}

	if nextPhase == nil || nextPlan == nil {
		t.Fatal("FindNextPlanJSON() expected to find next plan, got nil")
	}

	// The key assertion for issue #14: next phase should be 3, NOT 2
	// Even though phase 2 directory exists with a pending plan, it's not in roadmap
	if nextPhase.Number != 3 {
		t.Errorf("FindNextPlanJSON() got phase %d, want phase 3 (stale phase 2 should be ignored)", nextPhase.Number)
	}

	if nextPlan.PlanNumber != "01" {
		t.Errorf("FindNextPlanJSON() got plan %s, want '01'", nextPlan.PlanNumber)
	}
}

// TestFindNextPlanJSON_CorrectPhaseAfterCompletion is a regression test for GitHub Issue #8
// Ensures that after completing phase 3, the next phase suggested is 4, not 2
func TestFindNextPlanJSON_CorrectPhaseAfterCompletion(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "ralph-test-next-phase-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	planningDir := tempDir
	phasesDir := filepath.Join(planningDir, "phases")
	os.MkdirAll(phasesDir, 0755)

	// Create roadmap.json with 4 phases - 1, 2, 3 complete, 4 pending
	roadmap := types.Roadmap{
		Version:     "1.0",
		ProjectName: "Test Project",
		Overview:    "Test",
		Phases: []types.Phase{
			{Number: 1, Name: "Phase One", Goal: "First phase", Status: types.StatusComplete},
			{Number: 2, Name: "Phase Two", Goal: "Second phase", Status: types.StatusComplete},
			{Number: 3, Name: "Phase Three", Goal: "Third phase", Status: types.StatusComplete},
			{Number: 4, Name: "Phase Four", Goal: "Fourth phase", Status: types.StatusPending},
		},
	}
	roadmapData, _ := json.MarshalIndent(roadmap, "", "  ")
	os.WriteFile(filepath.Join(planningDir, "roadmap.json"), roadmapData, 0644)

	// Create phase directories
	phase1Dir := filepath.Join(phasesDir, "01-phase-one")
	phase2Dir := filepath.Join(phasesDir, "02-phase-two")
	phase3Dir := filepath.Join(phasesDir, "03-phase-three")
	phase4Dir := filepath.Join(phasesDir, "04-phase-four")
	os.MkdirAll(phase1Dir, 0755)
	os.MkdirAll(phase2Dir, 0755)
	os.MkdirAll(phase3Dir, 0755)
	os.MkdirAll(phase4Dir, 0755)

	// Create completed plans in phases 1-3
	for i, phaseDir := range []string{phase1Dir, phase2Dir, phase3Dir} {
		plan := &types.Plan{
			Phase:      phaseDir,
			PlanNumber: "01",
			Status:     types.StatusComplete,
			Objective:  "Completed Plan",
			Tasks: []types.Task{{
				ID: "task-1", Name: "Task 1", Type: types.TaskTypeAuto,
				Action: "Do task", Verify: "verify", Done: "done", Status: types.StatusComplete,
			}},
			Verification: []string{},
			CreatedAt:    time.Now(),
		}
		SavePlanJSON(filepath.Join(phaseDir, "0"+string(rune('1'+i))+"-01.json"), plan)
	}

	// Create pending plan in phase 4
	plan4 := &types.Plan{
		Phase:      "04-phase-four",
		PlanNumber: "01",
		Status:     types.StatusPending,
		Objective:  "Phase 4 Plan - Should be next",
		Tasks: []types.Task{{
			ID: "task-1", Name: "Task 1", Type: types.TaskTypeAuto,
			Action: "Do task", Verify: "verify", Done: "done", Status: types.StatusPending,
		}},
		Verification: []string{},
		CreatedAt:    time.Now(),
	}
	SavePlanJSON(filepath.Join(phase4Dir, "04-01.json"), plan4)

	// Find next plan - should be phase 4, NOT phase 2
	nextPhase, nextPlan, err := FindNextPlanJSON(planningDir)
	if err != nil {
		t.Fatalf("FindNextPlanJSON() unexpected error = %v", err)
	}

	if nextPhase == nil || nextPlan == nil {
		t.Fatal("FindNextPlanJSON() expected to find next plan, got nil")
	}

	// The key assertion for issue #8: next phase after completing 1,2,3 should be 4
	if nextPhase.Number != 4 {
		t.Errorf("FindNextPlanJSON() got phase %d, want phase 4 (after completing phases 1-3)", nextPhase.Number)
	}

	if nextPlan.Objective != "Phase 4 Plan - Should be next" {
		t.Errorf("FindNextPlanJSON() got wrong plan: %s", nextPlan.Objective)
	}
}

// TestFindPhaseDirByNumber tests that phase directories are found correctly by number
// This supports the fix for #14 by ensuring we match directories by number prefix
func TestFindPhaseDirByNumber(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "ralph-test-phasedir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	phasesDir := filepath.Join(tempDir, "phases")
	os.MkdirAll(phasesDir, 0755)

	// Create phase directories with various names
	os.MkdirAll(filepath.Join(phasesDir, "01-first-phase"), 0755)
	os.MkdirAll(filepath.Join(phasesDir, "02-renamed-phase"), 0755) // Name changed from original
	os.MkdirAll(filepath.Join(phasesDir, "03-third-phase"), 0755)

	t.Run("finds phase 1 by number", func(t *testing.T) {
		result := FindPhaseDirByNumber(tempDir, 1)
		if result == "" {
			t.Error("FindPhaseDirByNumber(1) expected to find directory, got empty")
		}
		if !filepath.IsAbs(result) {
			t.Error("FindPhaseDirByNumber should return absolute path")
		}
	})

	t.Run("finds phase 2 despite renamed directory", func(t *testing.T) {
		result := FindPhaseDirByNumber(tempDir, 2)
		if result == "" {
			t.Error("FindPhaseDirByNumber(2) expected to find directory, got empty")
		}
		expected := filepath.Join(phasesDir, "02-renamed-phase")
		if result != expected {
			t.Errorf("FindPhaseDirByNumber(2) = %s, want %s", result, expected)
		}
	})

	t.Run("returns empty for non-existent phase", func(t *testing.T) {
		result := FindPhaseDirByNumber(tempDir, 99)
		if result != "" {
			t.Errorf("FindPhaseDirByNumber(99) expected empty, got %s", result)
		}
	})
}
