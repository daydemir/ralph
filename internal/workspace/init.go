package workspace

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daydemir/ralph/internal/prompts"
)

// Init creates a new Ralph workspace in the current directory
func Init(force bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	ralphPath := filepath.Join(cwd, RalphDir)

	// Check if workspace already exists
	if _, err := os.Stat(ralphPath); err == nil {
		if !force {
			return ErrWorkspaceExists
		}
		// Remove existing workspace if force
		if err := os.RemoveAll(ralphPath); err != nil {
			return fmt.Errorf("failed to remove existing workspace: %w", err)
		}
	}

	// Create directory structure
	dirs := []string{
		ralphPath,
		filepath.Join(ralphPath, "prompts"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create config.yaml
	if err := writeFile(filepath.Join(ralphPath, "config.yaml"), defaultConfig); err != nil {
		return err
	}

	// Create prd.json
	if err := writeFile(filepath.Join(ralphPath, "prd.json"), defaultPRD); err != nil {
		return err
	}

	// Create prd-completed.json
	if err := writeFile(filepath.Join(ralphPath, "prd-completed.json"), emptyPRD); err != nil {
		return err
	}

	// Create codebase-map.md
	if err := writeFile(filepath.Join(ralphPath, "codebase-map.md"), defaultCodebaseMap); err != nil {
		return err
	}

	// Create progress.txt
	if err := writeFile(filepath.Join(ralphPath, "progress.txt"), defaultProgress); err != nil {
		return err
	}

	// Create fix_plan.md
	if err := writeFile(filepath.Join(ralphPath, "fix_plan.md"), defaultFixPlan); err != nil {
		return err
	}

	// Copy prompt templates
	if err := copyPrompts(filepath.Join(ralphPath, "prompts")); err != nil {
		return err
	}

	fmt.Println("Initialized Ralph workspace in", ralphPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit .ralph/codebase-map.md with your project structure")
	fmt.Println("  2. Run 'ralph plan' to create PRDs")
	fmt.Println("  3. Run 'ralph build' to execute them")

	return nil
}

func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func copyPrompts(promptsDir string) error {
	promptFiles := []string{"plan.md", "build.md"}
	for _, name := range promptFiles {
		content, err := prompts.Get(name)
		if err != nil {
			return fmt.Errorf("failed to get embedded prompt %s: %w", name, err)
		}
		path := filepath.Join(promptsDir, name)
		if err := writeFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// Embed FS placeholder - will be used by prompts package
var _ embed.FS

const defaultConfig = `# Ralph configuration
llm:
  backend: claude          # claude | kilocode
  model: sonnet

claude:
  binary: claude           # Path to Claude Code CLI
  allowed_tools:
    - Read
    - Write
    - Edit
    - Bash
    - Glob
    - Grep
    - Task
    - TodoWrite
    - WebFetch
    - WebSearch

mistral:
  binary: vibe             # Path to Vibe CLI
  api_key: ""              # Your Mistral API key (set with: ralph config set mistral.api_key "xxx")

build:
  default_loop_iterations: 10
  signals:
    iteration_complete: "###ITERATION_COMPLETE###"
    ralph_complete: "###RALPH_COMPLETE###"
`

const defaultPRD = `{
  "features": []
}
`

const emptyPRD = `{
  "features": []
}
`

const defaultCodebaseMap = `# Codebase Map

Describe your project structure for Ralph to understand your codebase.

## Repositories

List your repositories relative to this workspace:

- ` + "`../my-app/`" + ` - Main application (language, framework)
- ` + "`../my-backend/`" + ` - Backend API (language, framework)

## Build & Test Commands

How to build and test each repo:

### my-app
- Build: ` + "`cd ../my-app && npm run build`" + `
- Test: ` + "`cd ../my-app && npm test`" + `
- Lint: ` + "`cd ../my-app && npm run lint`" + `

## Tech Stack

Key technologies:
- Frontend: (React, Vue, etc.)
- Backend: (Node.js, Python, etc.)
- Database: (PostgreSQL, MongoDB, etc.)

## Important Files

Key files Ralph should know about:
- ` + "`../my-app/src/index.ts`" + ` - Entry point
- ` + "`../my-app/src/api/`" + ` - API routes
`

const defaultProgress = `# Ralph Progress

This file tracks completed work and learnings from Ralph sessions.
`

const defaultFixPlan = `# Fix Plan

Known issues and bugs to address:

## Active Issues

(none yet)

## Resolved

(none yet)
`
