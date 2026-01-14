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

	if obs.Type != "bug" {
		t.Errorf("Expected type 'bug', got '%s'", obs.Type)
	}

	if obs.Title != "iOS 18 availability required for SpatialAudioComponent" {
		t.Errorf("Expected title 'iOS 18 availability required for SpatialAudioComponent', got '%s'", obs.Title)
	}

	if obs.Action != "none" {
		t.Errorf("Expected action 'none' for auto-fixed issue, got '%s'", obs.Action)
	}

	if obs.File == "" {
		t.Error("Expected file path to be extracted")
	}

	t.Logf("Parsed observation: Type=%s, Title=%s, File=%s, Action=%s, Detail=%s",
		obs.Type, obs.Title, obs.File, obs.Action, obs.Detail)
}

func TestParseObservationsXML(t *testing.T) {
	content := `
## Observations

<observation type="stub" severity="medium">
  <title>3 backend tests are stubs</title>
  <detail>image.test.ts and video.test.ts have stub tests</detail>
  <file>mix-backend/functions/src/__tests__/endpoints/</file>
  <action>needs-implementation</action>
</observation>
`

	observations := ParseObservations(content)

	if len(observations) != 1 {
		t.Fatalf("Expected 1 observation, got %d", len(observations))
	}

	obs := observations[0]

	if obs.Type != "stub" {
		t.Errorf("Expected type 'stub', got '%s'", obs.Type)
	}

	if obs.Title != "3 backend tests are stubs" {
		t.Errorf("Expected title '3 backend tests are stubs', got '%s'", obs.Title)
	}

	if obs.Action != "needs-implementation" {
		t.Errorf("Expected action 'needs-implementation', got '%s'", obs.Action)
	}
}
