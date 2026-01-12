package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.md
var embeddedPrompts embed.FS

// Get returns the prompt content, preferring local .ralph/prompts/ over embedded
func Get(name string) (string, error) {
	// Normalize name
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	// Try embedded prompts
	content, err := embeddedPrompts.ReadFile("templates/" + name)
	if err != nil {
		return "", fmt.Errorf("prompt %s not found: %w", name, err)
	}
	return string(content), nil
}

// GetForWorkspace returns prompt content, checking workspace first then embedded
func GetForWorkspace(workspaceDir, name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	// Try workspace prompts first
	localPath := filepath.Join(workspaceDir, ".ralph", "prompts", name)
	if content, err := os.ReadFile(localPath); err == nil {
		return string(content), nil
	}

	// Fall back to embedded
	return Get(name)
}
