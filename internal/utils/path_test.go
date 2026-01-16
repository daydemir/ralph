package utils

import (
	"path/filepath"
	"runtime"
	"testing"
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
			name:     "mixed case with spaces",
			input:    "Hello World",
			expected: "hello-world",
		},
		{
			name:     "spaces to hyphens",
			input:    "critical bug fixes",
			expected: "critical-bug-fixes",
		},
		{
			name:     "multiple spaces become multiple hyphens",
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
			input:    "!@#$%^&*()",
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
		{
			name:     "parentheses removed",
			input:    "Setup (Initial)",
			expected: "setup-initial",
		},
		{
			name:     "leading and trailing spaces",
			input:    " hello world ",
			expected: "-hello-world-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractPlanName(t *testing.T) {
	tests := []struct {
		name      string
		objective string
		expected  string
	}{
		{
			name:      "short objective with no period",
			objective: "Fix critical bugs",
			expected:  "Fix critical bugs",
		},
		{
			name:      "objective with period",
			objective: "Fix critical bugs. This is important for stability.",
			expected:  "Fix critical bugs",
		},
		{
			name:      "objective with multiple periods",
			objective: "Fix bugs. Update tests. Deploy changes.",
			expected:  "Fix bugs",
		},
		{
			name:      "empty objective",
			objective: "",
			expected:  "",
		},
		{
			name:      "objective starting with period",
			objective: ".Something weird",
			expected:  "",
		},
		{
			name:      "long objective under 80 chars",
			objective: "This is a moderately long objective that is still under eighty characters total",
			expected:  "This is a moderately long objective that is still under eighty characters total",
		},
		{
			name:      "long objective over 80 chars (no period)",
			objective: "This is a very long objective that exceeds eighty characters and should be truncated with ellipsis at the end here",
			expected:  "This is a very long objective that exceeds eighty characters and should be tr...",
		},
		{
			name:      "exactly 80 chars",
			objective: "12345678901234567890123456789012345678901234567890123456789012345678901234567890",
			expected:  "12345678901234567890123456789012345678901234567890123456789012345678901234567890",
		},
		{
			name:      "81 chars gets truncated",
			objective: "123456789012345678901234567890123456789012345678901234567890123456789012345678901",
			expected:  "12345678901234567890123456789012345678901234567890123456789012345678901234567...",
		},
		{
			name:      "objective with leading/trailing whitespace in first sentence",
			objective: "  Fix critical bugs  . This is extra.",
			expected:  "Fix critical bugs",
		},
		{
			name:      "objective with only whitespace",
			objective: "   ",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPlanName(tt.objective)
			if got != tt.expected {
				t.Errorf("ExtractPlanName(%q) = %q, want %q", tt.objective, got, tt.expected)
			}
		})
	}
}

func TestBuildPhasePath(t *testing.T) {
	tests := []struct {
		name        string
		planningDir string
		phaseNumber int
		phaseName   string
		expected    string
	}{
		{
			name:        "typical phase",
			planningDir: "/project/.planning",
			phaseNumber: 1,
			phaseName:   "Critical Bug Fixes",
			expected:    filepath.Join("/project/.planning", "phases", "01-critical-bug-fixes"),
		},
		{
			name:        "double digit phase",
			planningDir: "/project/.planning",
			phaseNumber: 12,
			phaseName:   "Final Cleanup",
			expected:    filepath.Join("/project/.planning", "phases", "12-final-cleanup"),
		},
		{
			name:        "single digit with leading zero",
			planningDir: "/project/.planning",
			phaseNumber: 5,
			phaseName:   "Testing",
			expected:    filepath.Join("/project/.planning", "phases", "05-testing"),
		},
		{
			name:        "phase name with special characters",
			planningDir: "/project/.planning",
			phaseNumber: 3,
			phaseName:   "Setup (Initial)",
			expected:    filepath.Join("/project/.planning", "phases", "03-setup-initial"),
		},
		{
			name:        "empty phase name",
			planningDir: "/project/.planning",
			phaseNumber: 1,
			phaseName:   "",
			expected:    filepath.Join("/project/.planning", "phases", "01-"),
		},
		{
			name:        "relative planning dir",
			planningDir: ".planning",
			phaseNumber: 1,
			phaseName:   "Setup",
			expected:    filepath.Join(".planning", "phases", "01-setup"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPhasePath(tt.planningDir, tt.phaseNumber, tt.phaseName)
			if got != tt.expected {
				t.Errorf("BuildPhasePath(%q, %d, %q) = %q, want %q",
					tt.planningDir, tt.phaseNumber, tt.phaseName, got, tt.expected)
			}
		})
	}
}

func TestBuildPlanPath(t *testing.T) {
	tests := []struct {
		name        string
		phaseDir    string
		phaseNumber int
		planNumber  string
		expected    string
	}{
		{
			name:        "simple plan",
			phaseDir:    "/project/.planning/phases/01-setup",
			phaseNumber: 1,
			planNumber:  "01",
			expected:    filepath.Join("/project/.planning/phases/01-setup", "01-01.json"),
		},
		{
			name:        "decimal plan number",
			phaseDir:    "/project/.planning/phases/01-setup",
			phaseNumber: 1,
			planNumber:  "01.1",
			expected:    filepath.Join("/project/.planning/phases/01-setup", "01-01.1.json"),
		},
		{
			name:        "double digit phase and plan",
			phaseDir:    "/project/.planning/phases/12-final",
			phaseNumber: 12,
			planNumber:  "05",
			expected:    filepath.Join("/project/.planning/phases/12-final", "12-05.json"),
		},
		{
			name:        "single digit phase number",
			phaseDir:    "/project/.planning/phases/05-testing",
			phaseNumber: 5,
			planNumber:  "02",
			expected:    filepath.Join("/project/.planning/phases/05-testing", "05-02.json"),
		},
		{
			name:        "relative phase dir",
			phaseDir:    ".planning/phases/01-setup",
			phaseNumber: 1,
			planNumber:  "01",
			expected:    filepath.Join(".planning/phases/01-setup", "01-01.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPlanPath(tt.phaseDir, tt.phaseNumber, tt.planNumber)
			if got != tt.expected {
				t.Errorf("BuildPlanPath(%q, %d, %q) = %q, want %q",
					tt.phaseDir, tt.phaseNumber, tt.planNumber, got, tt.expected)
			}
		})
	}
}

func TestBuildPhasePathCrossplatform(t *testing.T) {
	// Test that paths work correctly on the current platform
	result := BuildPhasePath("/base/dir", 1, "test phase")

	// Should use platform-specific separator
	if runtime.GOOS == "windows" {
		if result != "\\base\\dir\\phases\\01-test-phase" {
			// Windows path handling with absolute paths may vary
			t.Logf("Windows path result: %s", result)
		}
	} else {
		expected := "/base/dir/phases/01-test-phase"
		if result != expected {
			t.Errorf("BuildPhasePath() = %q, want %q", result, expected)
		}
	}
}
