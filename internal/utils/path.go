package utils

import "strings"

// Slugify converts a name to a directory-safe slug
// Example: "Critical Bug Fixes" -> "critical-bug-fixes"
func Slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	result := ""
	for _, c := range slug {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result += string(c)
		}
	}
	return result
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
