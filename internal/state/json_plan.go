package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/daydemir/ralph/internal/types"
)

// LoadPlanJSON loads a plan from a JSON file (e.g., .planning/phases/01-name/01-01.json)
// Returns error if file not found, malformed JSON, or validation fails
func LoadPlanJSON(planPath string) (*types.Plan, error) {
	file, err := os.Open(planPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open plan file: %w", err)
	}
	defer file.Close()

	var plan types.Plan
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Reject unknown fields for strict validation

	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("cannot decode plan JSON: %w", err)
	}

	// Validate the loaded plan using detailed validation
	// This returns structured errors that can be used for self-healing
	validationErrs := plan.ValidateWithDetails()
	if validationErrs.HasErrors() {
		return nil, validationErrs
	}

	return &plan, nil
}

// SavePlanJSON saves a plan to a JSON file atomically
// Validates before writing - no silent failures
func SavePlanJSON(planPath string, plan *types.Plan) error {
	// Validate before writing
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid plan: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal plan: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := planPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp plan file: %w", err)
	}

	if err := os.Rename(tempPath, planPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("cannot rename temp plan file: %w", err)
	}

	return nil
}

// LoadAllPlansJSON loads all plans from a phase directory
// Scans for *.json files, sorts by plan number
func LoadAllPlansJSON(phaseDir string) ([]types.Plan, error) {
	entries, err := os.ReadDir(phaseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read phase directory: %w", err)
	}

	var plans []types.Plan
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Only load .json files
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		planPath := filepath.Join(phaseDir, name)
		plan, err := LoadPlanJSON(planPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load plan %s: %w", name, err)
		}

		plans = append(plans, *plan)
	}

	// Sort by plan number (supports both integer and decimal like "01" and "01.1")
	sort.Slice(plans, func(i, j int) bool {
		ni, _ := strconv.ParseFloat(plans[i].PlanNumber, 64)
		nj, _ := strconv.ParseFloat(plans[j].PlanNumber, 64)
		return ni < nj
	})

	return plans, nil
}

// FindNextPlanJSON finds the first incomplete plan across all phases
// Returns the phase and plan, or (nil, nil) if all plans are complete
func FindNextPlanJSON(planningDir string) (*types.Phase, *types.Plan, error) {
	// Load roadmap to get phases
	roadmap, err := LoadRoadmapJSON(planningDir)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot load roadmap: %w", err)
	}

	// Iterate through phases
	for i := range roadmap.Phases {
		phase := &roadmap.Phases[i]

		// Load all plans for this phase
		phaseDir := filepath.Join(planningDir, "phases",
			fmt.Sprintf("%02d-%s", phase.Number, slugify(phase.Name)))

		plans, err := LoadAllPlansJSON(phaseDir)
		if err != nil {
			// Skip phases with no plans directory
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, fmt.Errorf("cannot load plans for phase %d: %w", phase.Number, err)
		}

		// Find first incomplete plan
		for j := range plans {
			plan := &plans[j]
			if plan.Status != types.StatusComplete {
				return phase, plan, nil
			}
		}
	}

	// All plans complete
	return nil, nil, nil
}

// slugify converts a phase name to a directory-safe slug
// Example: "Critical Bug Fixes" -> "critical-bug-fixes"
func slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove any non-alphanumeric characters except hyphens
	result := ""
	for _, c := range slug {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result += string(c)
		}
	}
	return result
}
