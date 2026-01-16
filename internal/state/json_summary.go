package state

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/daydemir/ralph/internal/types"
)

// LoadSummaryJSON loads summary from a summary.json file
// Returns error if file not found, malformed JSON, or validation fails
func LoadSummaryJSON(summaryPath string) (*types.Summary, error) {
	file, err := os.Open(summaryPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open summary.json: %w", err)
	}
	defer file.Close()

	var summary types.Summary
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&summary); err != nil {
		return nil, fmt.Errorf("cannot decode summary.json: %w", err)
	}

	// Validate the loaded summary
	if err := summary.Validate(); err != nil {
		return nil, fmt.Errorf("summary validation failed: %w", err)
	}

	return &summary, nil
}

// SaveSummaryJSON saves summary to a JSON file atomically
// Validates before writing - no silent failures
func SaveSummaryJSON(summaryPath string, summary *types.Summary) error {
	// Validate before writing
	if err := summary.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid summary: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal summary: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := summaryPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp summary file: %w", err)
	}

	if err := os.Rename(tempPath, summaryPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp summary file: %w", err)
	}

	return nil
}

// SummaryPathFromPlan returns the summary path for a plan
// Replaces .json with -summary.json
func SummaryPathFromPlan(planPath string) string {
	return strings.Replace(planPath, ".json", "-summary.json", 1)
}

// SummaryExists checks if summary.json exists for a plan
func SummaryExists(planPath string) bool {
	summaryPath := SummaryPathFromPlan(planPath)
	_, err := os.Stat(summaryPath)
	return err == nil
}
