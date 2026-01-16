package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed templates/*
var embeddedPrompts embed.FS

// processAtReferences resolves @-references in prompt content
// Supports: @path/to/file.md -> loads and inlines that file
// Prevents circular references with a visited map
func processAtReferences(content string, basePath string, visited map[string]bool) (string, error) {
	if visited == nil {
		visited = make(map[string]bool)
	}

	// Match @path/to/file.md patterns (not inside code blocks)
	atRefPattern := regexp.MustCompile(`(?m)^@([^\s]+\.md)\s*$`)

	result := atRefPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the path (remove @ prefix and whitespace)
		refPath := strings.TrimPrefix(strings.TrimSpace(match), "@")

		// Check for circular reference
		if visited[refPath] {
			return fmt.Sprintf("<!-- CIRCULAR REFERENCE: %s -->", refPath)
		}
		visited[refPath] = true

		// Try to load the referenced file
		var refContent string
		var err error

		// First try relative to basePath (workspace prompts)
		if basePath != "" {
			localPath := filepath.Join(basePath, refPath)
			if data, readErr := os.ReadFile(localPath); readErr == nil {
				refContent = string(data)
			}
		}

		// Fall back to embedded prompts
		if refContent == "" {
			data, readErr := embeddedPrompts.ReadFile("templates/" + refPath)
			if readErr != nil {
				err = readErr
			} else {
				refContent = string(data)
			}
		}

		if err != nil {
			return fmt.Sprintf("<!-- REFERENCE NOT FOUND: %s -->", refPath)
		}

		// Recursively process references in the included content
		processed, processErr := processAtReferences(refContent, basePath, visited)
		if processErr != nil {
			return fmt.Sprintf("<!-- ERROR PROCESSING: %s: %v -->", refPath, processErr)
		}

		return processed
	})

	return result, nil
}

// Get returns the prompt content from embedded templates
func Get(name string) (string, error) {
	// Normalize name - support both with and without .md extension
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	// Try embedded prompts
	content, err := embeddedPrompts.ReadFile("templates/" + name)
	if err != nil {
		return "", fmt.Errorf("prompt %s not found: %w", name, err)
	}

	// Process @-references
	processed, err := processAtReferences(string(content), "", nil)
	if err != nil {
		return "", fmt.Errorf("error processing references in %s: %w", name, err)
	}

	return processed, nil
}

// GetForWorkspace returns prompt content, checking workspace first then embedded
// Supports override via .ralph/prompts/ directory
func GetForWorkspace(workspaceDir, name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	var content string
	var basePath string

	// Try workspace prompts first (allows overrides)
	localPath := filepath.Join(workspaceDir, ".ralph", "prompts", name)
	if data, err := os.ReadFile(localPath); err == nil {
		content = string(data)
		basePath = filepath.Join(workspaceDir, ".ralph", "prompts")
	}

	// Fall back to embedded
	if content == "" {
		data, err := embeddedPrompts.ReadFile("templates/" + name)
		if err != nil {
			return "", fmt.Errorf("prompt %s not found in workspace or embedded: %w", name, err)
		}
		content = string(data)
	}

	// Process @-references with workspace path for local overrides
	processed, err := processAtReferences(content, basePath, nil)
	if err != nil {
		return "", fmt.Errorf("error processing references in %s: %w", name, err)
	}

	return processed, nil
}

// GetAgent returns an agent prompt (shorthand for agents/name.md)
func GetAgent(name string) (string, error) {
	return Get("agents/" + name)
}

// GetAgentForWorkspace returns an agent prompt with workspace override support
func GetAgentForWorkspace(workspaceDir, name string) (string, error) {
	return GetForWorkspace(workspaceDir, "agents/"+name)
}

// GetReference returns a reference document (shorthand for references/name.md)
func GetReference(name string) (string, error) {
	return Get("references/" + name)
}

// GetReferenceForWorkspace returns a reference with workspace override support
func GetReferenceForWorkspace(workspaceDir, name string) (string, error) {
	return GetForWorkspace(workspaceDir, "references/"+name)
}

// GetWorkflow returns a workflow prompt (shorthand for workflows/name.md)
func GetWorkflow(name string) (string, error) {
	return Get("workflows/" + name)
}

// GetWorkflowForWorkspace returns a workflow with workspace override support
func GetWorkflowForWorkspace(workspaceDir, name string) (string, error) {
	return GetForWorkspace(workspaceDir, "workflows/"+name)
}

// ListAvailable returns all available prompt names (embedded)
func ListAvailable() ([]string, error) {
	var prompts []string

	err := walkEmbeddedDir("templates", func(path string) {
		// Remove "templates/" prefix and return relative path
		relPath := strings.TrimPrefix(path, "templates/")
		prompts = append(prompts, relPath)
	})

	return prompts, err
}

// walkEmbeddedDir walks the embedded filesystem directory
func walkEmbeddedDir(dir string, fn func(path string)) error {
	entries, err := embeddedPrompts.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := dir + "/" + entry.Name()
		if entry.IsDir() {
			if err := walkEmbeddedDir(path, fn); err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".md") {
			fn(path)
		}
	}

	return nil
}

// Exists checks if a prompt exists (in embedded or workspace)
func Exists(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	_, err := embeddedPrompts.ReadFile("templates/" + name)
	return err == nil
}

// ExistsInWorkspace checks if a prompt exists in workspace override directory
func ExistsInWorkspace(workspaceDir, name string) bool {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	localPath := filepath.Join(workspaceDir, ".ralph", "prompts", name)
	_, err := os.Stat(localPath)
	return err == nil
}
