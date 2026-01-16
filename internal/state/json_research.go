package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

// LoadResearchJSON loads research from a research.json file
// Returns error if file not found, malformed JSON, or validation fails
func LoadResearchJSON(researchPath string) (*types.Research, error) {
	file, err := os.Open(researchPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open research.json: %w", err)
	}
	defer file.Close()

	var research types.Research
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&research); err != nil {
		return nil, fmt.Errorf("cannot decode research.json: %w", err)
	}

	// Validate the loaded research
	if err := research.Validate(); err != nil {
		return nil, fmt.Errorf("research validation failed: %w", err)
	}

	return &research, nil
}

// SaveResearchJSON saves research to a JSON file atomically
// Validates before writing - no silent failures
func SaveResearchJSON(researchPath string, research *types.Research) error {
	// Validate before writing
	if err := research.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid research: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(research, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal research: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := researchPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp research file: %w", err)
	}

	if err := os.Rename(tempPath, researchPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp research file: %w", err)
	}

	return nil
}

// ResearchPathForPhase returns the research.json path for a phase directory
func ResearchPathForPhase(phaseDir string, phaseNumber int) string {
	return filepath.Join(phaseDir, fmt.Sprintf("%02d-research.json", phaseNumber))
}

// InitResearchJSON creates an initial research.json for a phase
func InitResearchJSON(phaseDir string, phaseNumber int, phaseName string) error {
	researchPath := ResearchPathForPhase(phaseDir, phaseNumber)

	// Check if research.json already exists
	if _, err := os.Stat(researchPath); err == nil {
		return fmt.Errorf("research.json already exists at %s", researchPath)
	}

	// Create initial research
	research := &types.Research{
		Version:        "1.0",
		Phase:          phaseNumber,
		PhaseName:      phaseName,
		DiscoveryLevel: 0,
		Summary:        "",
		Recommendation: "",
		KeyFindings:    []string{},
		Risks:          []types.Risk{},
		Options:        []types.ResearchOption{},
		CreatedAt:      time.Now(),
	}

	return SaveResearchJSON(researchPath, research)
}

// ResearchExists checks if research.json exists for a phase
func ResearchExists(phaseDir string, phaseNumber int) bool {
	researchPath := ResearchPathForPhase(phaseDir, phaseNumber)
	_, err := os.Stat(researchPath)
	return err == nil
}
