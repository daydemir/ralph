package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

// LoadContextJSON loads context from a context.json file
// Returns error if file not found, malformed JSON, or validation fails
func LoadContextJSON(contextPath string) (*types.Context, error) {
	file, err := os.Open(contextPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open context.json: %w", err)
	}
	defer file.Close()

	var context types.Context
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&context); err != nil {
		return nil, fmt.Errorf("cannot decode context.json: %w", err)
	}

	// Validate the loaded context
	if err := context.Validate(); err != nil {
		return nil, fmt.Errorf("context validation failed: %w", err)
	}

	return &context, nil
}

// SaveContextJSON saves context to a JSON file atomically
// Validates before writing - no silent failures
func SaveContextJSON(contextPath string, context *types.Context) error {
	// Validate before writing
	if err := context.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid context: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal context: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := contextPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp context file: %w", err)
	}

	if err := os.Rename(tempPath, contextPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp context file: %w", err)
	}

	return nil
}

// ContextPathForPhase returns the context.json path for a phase directory
func ContextPathForPhase(phaseDir string, phaseNumber int) string {
	return filepath.Join(phaseDir, fmt.Sprintf("%02d-context.json", phaseNumber))
}

// InitContextJSON creates an initial context.json for a phase
func InitContextJSON(phaseDir string, phaseNumber int, phaseName string) error {
	contextPath := ContextPathForPhase(phaseDir, phaseNumber)

	// Check if context.json already exists
	if _, err := os.Stat(contextPath); err == nil {
		return fmt.Errorf("context.json already exists at %s", contextPath)
	}

	// Create initial context
	context := &types.Context{
		Version:           "1.0",
		Phase:             phaseNumber,
		PhaseName:         phaseName,
		DiscussionSummary: "",
		UserVision:        []string{},
		Requirements:      []string{},
		Constraints:       []string{},
		Preferences:       []string{},
		MustHaves:         []string{},
		NiceToHaves:       []string{},
		CreatedAt:         time.Now(),
	}

	return SaveContextJSON(contextPath, context)
}

// ContextExists checks if context.json exists for a phase
func ContextExists(phaseDir string, phaseNumber int) bool {
	contextPath := ContextPathForPhase(phaseDir, phaseNumber)
	_, err := os.Stat(contextPath)
	return err == nil
}
