package utils

import (
	"os"
	"strings"
)

// Slugify converts a name to a directory-safe slug
// Example: "Critical Bug Fixes" -> "critical-bug-fixes"
func Slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	result.Grow(len(slug))
	for _, c := range slug {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ExtractPlanName extracts a short name from the plan objective
// Takes first sentence or first 80 chars
func ExtractPlanName(objective string) string {
	lines := strings.Split(objective, ".")
	if len(lines) > 0 {
		name := strings.TrimSpace(lines[0])
		if len(name) > 80 {
			name = name[:77] + "..."
		}
		return name
	}
	return objective
}
