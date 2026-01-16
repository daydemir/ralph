package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

// LoadProjectJSON loads project from .planning/project.json
// Returns error if file not found, malformed JSON, or validation fails
func LoadProjectJSON(planningDir string) (*types.Project, error) {
	projectPath := filepath.Join(planningDir, "project.json")

	file, err := os.Open(projectPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open project.json: %w", err)
	}
	defer file.Close()

	var project types.Project
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&project); err != nil {
		return nil, fmt.Errorf("cannot decode project.json: %w", err)
	}

	// Validate the loaded project
	if err := project.Validate(); err != nil {
		return nil, fmt.Errorf("project validation failed: %w", err)
	}

	return &project, nil
}

// SaveProjectJSON saves project to .planning/project.json atomically
// Validates before writing - no silent failures
func SaveProjectJSON(planningDir string, project *types.Project) error {
	// Validate before writing
	if err := project.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid project: %w", err)
	}

	projectPath := filepath.Join(planningDir, "project.json")

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal project: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := projectPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp project file: %w", err)
	}

	if err := os.Rename(tempPath, projectPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp project file: %w", err)
	}

	return nil
}

// InitProjectJSON creates initial project.json with default values
// Returns error if project.json already exists or cannot be created
func InitProjectJSON(planningDir string, name, description string) error {
	projectPath := filepath.Join(planningDir, "project.json")

	// Check if project.json already exists
	if _, err := os.Stat(projectPath); err == nil {
		return fmt.Errorf("project.json already exists at %s", projectPath)
	}

	// Create initial project
	now := time.Now()
	project := &types.Project{
		Version:     "1.0",
		Name:        name,
		Description: description,
		Goals:       []string{},
		TechStack:   []string{},
		Constraints: []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Save the initial project
	return SaveProjectJSON(planningDir, project)
}

// ProjectExists checks if project.json exists in the planning directory
func ProjectExists(planningDir string) bool {
	projectPath := filepath.Join(planningDir, "project.json")
	_, err := os.Stat(projectPath)
	return err == nil
}
