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
	state := &types.ProjectState{
		Version:      "1.0",
		CurrentPhase: 1,
		LastUpdated:  time.Now(),
	}

	// Save the initial state
	return SaveStateJSON(planningDir, state)
}
