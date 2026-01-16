package utils

import (
	"fmt"
	"path/filepath"
)

// BuildPhasePath builds the filesystem path for a phase directory
// Format: {planningDir}/phases/{NN}-{slugified-name}
func BuildPhasePath(planningDir string, phaseNumber int, phaseName string) string {
	return filepath.Join(planningDir, "phases",
		fmt.Sprintf("%02d-%s", phaseNumber, Slugify(phaseName)))
}

// BuildPlanPath builds the filesystem path for a plan JSON file
// Format: {phaseDir}/{NN}-{planNumber}.json
func BuildPlanPath(phaseDir string, phaseNumber int, planNumber string) string {
	return filepath.Join(phaseDir,
		fmt.Sprintf("%02d-%s.json", phaseNumber, planNumber))
}
