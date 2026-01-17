package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

// LoadStateJSON loads project state from .planning/state.json
// Returns error if file not found, malformed JSON, or validation fails
func LoadStateJSON(planningDir string) (*types.ProjectState, error) {
	statePath := filepath.Join(planningDir, "state.json")

	file, err := os.Open(statePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open state.json: %w", err)
	}
	defer file.Close()

	var state types.ProjectState
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("cannot decode state.json: %w", err)
	}

	// Validate the loaded state
	if err := state.Validate(); err != nil {
		return nil, fmt.Errorf("state validation failed: %w", err)
	}

	return &state, nil
}

// SaveStateJSON saves project state to .planning/state.json atomically
// Validates before writing - no silent failures
func SaveStateJSON(planningDir string, state *types.ProjectState) error {
	// Validate before writing
	if err := state.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid state: %w", err)
	}

	statePath := filepath.Join(planningDir, "state.json")

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal state: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp state file: %w", err)
	}

	return nil
}

// InitStateJSON creates initial state.json with default values
// Returns error if state.json already exists or cannot be created
func InitStateJSON(planningDir string) error {
	statePath := filepath.Join(planningDir, "state.json")

	// Check if state.json already exists
	if _, err := os.Stat(statePath); err == nil {
		return fmt.Errorf("state.json already exists at %s", statePath)
	}

	// Create initial state
	// Note: CurrentPhase is deprecated - use DeriveCurrentPhase() instead
	state := &types.ProjectState{
		Version:      "1.0",
		CurrentPhase: 1,
		LastUpdated:  time.Now(),
	}

	// Save the initial state
	return SaveStateJSON(planningDir, state)
}

// DeriveCurrentPhase scans roadmap.json to determine the current phase
// This is the source of truth - don't rely on state.json's CurrentPhase field
// Returns the number of the first incomplete phase, or the last phase if all complete
func DeriveCurrentPhase(planningDir string) (int, error) {
	roadmap, err := LoadRoadmapJSON(planningDir)
	if err != nil {
		return 0, fmt.Errorf("cannot load roadmap: %w", err)
	}

	if len(roadmap.Phases) == 0 {
		return 0, nil
	}

	// Find the first incomplete phase
	for _, phase := range roadmap.Phases {
		if phase.Status != types.StatusComplete {
			return phase.Number, nil
		}
	}

	// All phases complete - return last phase number
	return roadmap.Phases[len(roadmap.Phases)-1].Number, nil
}
