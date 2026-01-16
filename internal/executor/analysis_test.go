package executor

import (
	"testing"
)

func TestParseSummaryObservations(t *testing.T) {
	// Actual content from 02-01-SUMMARY.md
	content := `# Phase 2 Plan 1: Media Foundation Fixes Summary

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] iOS 18 availability required for SpatialAudioComponent**
- **Found during:** Task 3 build verification
- **Issue:** ` + "`SpatialAudioComponent`" + ` is iOS 18+ API, project targets iOS 17
- **Fix:** Wrapped configuration in ` + "`if #available(iOS 18.0, *)`" + ` guard
- **Files modified:** ` + "`ar/AR/MIX iOS/Model/RealityVisual.swift`" + `
- **Verification:** Build succeeded after adding guard
- **Commit:** 2cb58d3

---

**Total deviations:** 1 auto-fixed (iOS availability guard)
`

	observations := ParseSummaryObservations(content)

	if len(observations) != 1 {
		t.Fatalf("Expected 1 observation, got %d", len(observations))
	}

	obs := observations[0]

	// Summary observations get mapped to simplified types
	if obs.Type != "finding" {
		t.Errorf("Expected type 'finding' (mapped from bug), got '%s'", obs.Type)
	}

	if obs.Title != "iOS 18 availability required for SpatialAudioComponent" {
		t.Errorf("Expected title 'iOS 18 availability required for SpatialAudioComponent', got '%s'", obs.Title)
	}

	if obs.File == "" {
		t.Error("Expected file path to be extracted")
	}

	t.Logf("Parsed observation: Type=%s, Title=%s, File=%s, Description=%s",
		obs.Type, obs.Title, obs.File, obs.Description)
}

func TestParseObservationsXML_LegacyFormat(t *testing.T) {
	// Test legacy format (with severity and action) - should still parse
	content := `
## Observations

<observation type="stub" severity="medium">
  <title>3 backend tests are stubs</title>
  <detail>image.test.ts and video.test.ts have stub tests</detail>
  <file>mix-backend/functions/src/__tests__/endpoints/</file>
  <action>needs-implementation</action>
</observation>
`

	observations := ParseObservations(content, nil)

	if len(observations) != 1 {
		t.Fatalf("Expected 1 observation, got %d", len(observations))
	}

	obs := observations[0]

	// Legacy type preserved
	if obs.Type != "stub" {
		t.Errorf("Expected type 'stub', got '%s'", obs.Type)
	}

	if obs.Title != "3 backend tests are stubs" {
		t.Errorf("Expected title '3 backend tests are stubs', got '%s'", obs.Title)
	}

	// Detail gets mapped to Description
	if obs.Description != "image.test.ts and video.test.ts have stub tests" {
		t.Errorf("Expected description 'image.test.ts and video.test.ts have stub tests', got '%s'", obs.Description)
	}
}

func TestParseObservationsXML_NewFormat(t *testing.T) {
	// Test new simplified format
	content := `
## Observations

<observation type="finding">
  <title>3 backend tests are stubs</title>
  <description>image.test.ts and video.test.ts have stub tests that need implementation</description>
  <file>mix-backend/functions/src/__tests__/endpoints/</file>
</observation>

<observation type="blocker">
  <title>API credentials required</title>
  <description>Need production API key to test external service integration</description>
</observation>

<observation type="completion">
  <title>Auth endpoints already exist</title>
  <description>Login and logout endpoints were implemented in a previous session</description>
  <file>src/api/auth/</file>
</observation>
`

	observations := ParseObservations(content, nil)

	if len(observations) != 3 {
		t.Fatalf("Expected 3 observations, got %d", len(observations))
	}

	// Test finding observation
	finding := observations[0]
	if finding.Type != "finding" {
		t.Errorf("Expected type 'finding', got '%s'", finding.Type)
	}
	if finding.Title != "3 backend tests are stubs" {
		t.Errorf("Expected title '3 backend tests are stubs', got '%s'", finding.Title)
	}

	// Test blocker observation
	blocker := observations[1]
	if blocker.Type != "blocker" {
		t.Errorf("Expected type 'blocker', got '%s'", blocker.Type)
	}
	if blocker.File != "" {
		t.Errorf("Expected empty file for blocker, got '%s'", blocker.File)
	}

	// Test completion observation
	completion := observations[2]
	if completion.Type != "completion" {
		t.Errorf("Expected type 'completion', got '%s'", completion.Type)
	}
}

func TestHasActionableObservations(t *testing.T) {
	tests := []struct {
		name     string
		obs      []Observation
		expected bool
	}{
		{
			name:     "empty list",
			obs:      []Observation{},
			expected: false,
		},
		{
			name: "only completions",
			obs: []Observation{
				{Type: "completion", Title: "Already done"},
			},
			expected: false,
		},
		{
			name: "has finding",
			obs: []Observation{
				{Type: "finding", Title: "Something interesting"},
			},
			expected: true,
		},
		{
			name: "has blocker",
			obs: []Observation{
				{Type: "blocker", Title: "Can't continue"},
			},
			expected: true,
		},
		{
			name: "mixed with finding",
			obs: []Observation{
				{Type: "completion", Title: "Already done"},
				{Type: "finding", Title: "Something interesting"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasActionableObservations(tt.obs)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterByType(t *testing.T) {
	observations := []Observation{
		{Type: "blocker", Title: "Blocker 1"},
		{Type: "finding", Title: "Finding 1"},
		{Type: "finding", Title: "Finding 2"},
		{Type: "completion", Title: "Completion 1"},
	}

	blockers := FilterByType(observations, "blocker")
	if len(blockers) != 1 {
		t.Errorf("Expected 1 blocker, got %d", len(blockers))
	}

	findings := FilterByType(observations, "finding")
	if len(findings) != 2 {
		t.Errorf("Expected 2 findings, got %d", len(findings))
	}

	completions := FilterByType(observations, "completion")
	if len(completions) != 1 {
		t.Errorf("Expected 1 completion, got %d", len(completions))
	}
}
