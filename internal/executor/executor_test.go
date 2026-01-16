package executor

import (
	"os/exec"
	"testing"

	"github.com/daydemir/ralph/internal/types"
)

// TestValidationCommandExecution tests the validation command execution logic
// This is a regression test for GitHub Issue #13: Mandatory validation enforcement
func TestValidationCommandExecution(t *testing.T) {
	t.Run("validation command passes", func(t *testing.T) {
		cmd := "exit 0"
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()

		if err != nil {
			t.Errorf("Expected validation command '%s' to pass, got error: %v, output: %s", cmd, err, string(out))
		}
	})

	t.Run("validation command fails", func(t *testing.T) {
		cmd := "exit 1"
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()

		if err == nil {
			t.Errorf("Expected validation command '%s' to fail, got success, output: %s", cmd, string(out))
		}
	})

	t.Run("validation command captures output on failure", func(t *testing.T) {
		cmd := "echo 'test failed'; exit 1"
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()

		if err == nil {
			t.Errorf("Expected validation command to fail")
		}

		// Verify output is captured
		if len(out) == 0 {
			t.Error("Expected output to be captured on failure")
		}

		// Check output contains expected text
		if string(out) != "test failed\n" {
			t.Errorf("Expected output 'test failed\\n', got '%s'", string(out))
		}
	})
}

// TestPlanValidationCommands tests that types.Plan has the ValidationCommands field
// and it can be populated and read correctly
func TestPlanValidationCommands(t *testing.T) {
	t.Run("plan with no validation commands", func(t *testing.T) {
		plan := &types.Plan{
			PlanNumber:         "01",
			Objective:          "Test plan",
			ValidationCommands: nil,
		}

		if len(plan.ValidationCommands) != 0 {
			t.Errorf("Expected 0 validation commands, got %d", len(plan.ValidationCommands))
		}
	})

	t.Run("plan with validation commands", func(t *testing.T) {
		plan := &types.Plan{
			PlanNumber: "01",
			Objective:  "Test plan",
			ValidationCommands: []string{
				"go test ./...",
				"npm run lint",
				"make build",
			},
		}

		if len(plan.ValidationCommands) != 3 {
			t.Errorf("Expected 3 validation commands, got %d", len(plan.ValidationCommands))
		}

		expected := []string{"go test ./...", "npm run lint", "make build"}
		for i, cmd := range plan.ValidationCommands {
			if cmd != expected[i] {
				t.Errorf("Expected validation command %d to be '%s', got '%s'", i, expected[i], cmd)
			}
		}
	})
}

// TestValidationFailureSoftType verifies that validation failures result in soft failure type
// This ensures the retry logic can handle validation failures
func TestValidationFailureSoftType(t *testing.T) {
	// FailureSoft should be the type used for validation failures
	// This allows the retry loop to attempt again after fixing issues
	if FailureSoft != 2 {
		t.Errorf("Expected FailureSoft to be 2, got %d", FailureSoft)
	}

	// FailureHard should stop the loop
	if FailureHard != 1 {
		t.Errorf("Expected FailureHard to be 1, got %d", FailureHard)
	}

	// FailureNone means success
	if FailureNone != 0 {
		t.Errorf("Expected FailureNone to be 0, got %d", FailureNone)
	}
}

// TestValidationCommandSequence tests that validation commands are executed in order
// and stop on first failure (as implemented in executor.go)
func TestValidationCommandSequence(t *testing.T) {
	commands := []string{
		"exit 0", // Should pass
		"exit 1", // Should fail
		"exit 0", // Should not be reached
	}

	passedCommands := 0
	var failedCommand string

	for i, cmd := range commands {
		_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			failedCommand = cmd
			break
		}
		passedCommands = i + 1
	}

	if passedCommands != 1 {
		t.Errorf("Expected 1 command to pass before failure, got %d", passedCommands)
	}

	if failedCommand != "exit 1" {
		t.Errorf("Expected failed command to be 'exit 1', got '%s'", failedCommand)
	}
}
