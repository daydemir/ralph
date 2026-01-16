package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daydemir/ralph/internal/types"
)

// LoadRoadmapJSON loads roadmap from .planning/roadmap.json
// Returns error if file not found, malformed JSON, or validation fails
func LoadRoadmapJSON(planningDir string) (*types.Roadmap, error) {
	roadmapPath := filepath.Join(planningDir, "roadmap.json")

	file, err := os.Open(roadmapPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open roadmap.json: %w", err)
	}
	defer file.Close()

	var roadmap types.Roadmap
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&roadmap); err != nil {
		return nil, fmt.Errorf("cannot decode roadmap.json: %w", err)
	}

	// Validate the loaded roadmap
	if err := roadmap.Validate(); err != nil {
		return nil, fmt.Errorf("roadmap validation failed: %w", err)
	}

	return &roadmap, nil
}

// SaveRoadmapJSON saves roadmap to .planning/roadmap.json atomically
// Validates before writing - no silent failures
func SaveRoadmapJSON(planningDir string, roadmap *types.Roadmap) error {
	// Validate before writing
	if err := roadmap.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid roadmap: %w", err)
	}

	roadmapPath := filepath.Join(planningDir, "roadmap.json")

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal roadmap: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := roadmapPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp roadmap file: %w", err)
	}

	if err := os.Rename(tempPath, roadmapPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp roadmap file: %w", err)
	}

	return nil
}
