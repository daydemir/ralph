package prd

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// GenerateID creates a PRD ID from a title
// Format: slugified-title-XXXX (4-char hex suffix)
// Example: "Auth Login" -> "auth-login-a1b2"
func GenerateID(title string) string {
	slug := slugify(title)

	// Truncate to 30 chars max (leaving room for 5-char suffix: -XXXX)
	if len(slug) > 30 {
		slug = slug[:30]
		// Don't end with a hyphen
		slug = strings.TrimSuffix(slug, "-")
	}

	suffix := randomHex(4)
	return slug + "-" + suffix
}

// slugify converts a title to a URL-friendly slug
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace non-alphanumeric characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")

	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")

	// Collapse multiple hyphens
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	return s
}

// randomHex generates a random hex string of the specified length
func randomHex(n int) string {
	bytes := make([]byte, (n+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a fixed string if crypto/rand fails
		return "0000"[:n]
	}
	return hex.EncodeToString(bytes)[:n]
}
