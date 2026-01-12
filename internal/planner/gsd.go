package planner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GSD wraps Get Shit Done slash commands via Claude Code CLI
type GSD struct {
	ClaudeBinary string
	WorkDir      string
}

// NewGSD creates a new GSD planner
func NewGSD(claudeBinary, workDir string) *GSD {
	if claudeBinary == "" {
		claudeBinary = "claude"
	}
	// Try to resolve the binary path
	resolved := resolveBinaryPath(claudeBinary)
	return &GSD{
		ClaudeBinary: resolved,
		WorkDir:      workDir,
	}
}

// resolveBinaryPath finds the claude binary, checking common locations
func resolveBinaryPath(binaryPath string) string {
	if filepath.IsAbs(binaryPath) {
		return binaryPath
	}

	if path, err := exec.LookPath(binaryPath); err == nil {
		return path
	}

	home, _ := os.UserHomeDir()
	commonPaths := []string{
		filepath.Join(home, ".claude", "local", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return binaryPath
}

// PlanningDir returns the path to .planning directory
func (g *GSD) PlanningDir() string {
	return filepath.Join(g.WorkDir, ".planning")
}

// HasProject checks if PROJECT.md exists
func (g *GSD) HasProject() bool {
	_, err := os.Stat(filepath.Join(g.PlanningDir(), "PROJECT.md"))
	return err == nil
}

// HasRoadmap checks if ROADMAP.md exists
func (g *GSD) HasRoadmap() bool {
	_, err := os.Stat(filepath.Join(g.PlanningDir(), "ROADMAP.md"))
	return err == nil
}

// HasState checks if STATE.md exists
func (g *GSD) HasState() bool {
	_, err := os.Stat(filepath.Join(g.PlanningDir(), "STATE.md"))
	return err == nil
}

// HasCodebaseMaps checks if codebase analysis exists
func (g *GSD) HasCodebaseMaps() bool {
	codebaseDir := filepath.Join(g.PlanningDir(), "codebase")
	if _, err := os.Stat(codebaseDir); os.IsNotExist(err) {
		return false
	}
	// Check for at least one map file
	files, _ := os.ReadDir(codebaseDir)
	return len(files) > 0
}

// RequireProject returns an error if PROJECT.md doesn't exist
func (g *GSD) RequireProject() error {
	if !g.HasProject() {
		return fmt.Errorf(`no PROJECT.md found

Ralph requires proper planning before execution.
Run 'ralph init' first to create your project.`)
	}
	return nil
}

// RequireRoadmap returns an error if ROADMAP.md doesn't exist
func (g *GSD) RequireRoadmap() error {
	if err := g.RequireProject(); err != nil {
		return err
	}
	if !g.HasRoadmap() {
		return fmt.Errorf(`no ROADMAP.md found

You need a roadmap before planning individual phases.
Run 'ralph roadmap' first to create your phase breakdown.`)
	}
	return nil
}

// RunCommand executes a GSD slash command interactively
func (g *GSD) RunCommand(ctx context.Context, command string) error {
	// Verify claude is available
	if _, err := exec.LookPath(g.ClaudeBinary); err != nil {
		return claudeNotFoundError()
	}

	// Build the command with dangerously-skip-permissions for autonomous execution
	args := []string{"--dangerously-skip-permissions", command}

	cmd := exec.CommandContext(ctx, g.ClaudeBinary, args...)
	cmd.Dir = g.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return claudeNotFoundError()
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// NewProject runs /gsd:new-project
func (g *GSD) NewProject(ctx context.Context) error {
	return g.RunCommand(ctx, "/gsd:new-project")
}

// CreateRoadmap runs /gsd:create-roadmap (requires PROJECT.md)
func (g *GSD) CreateRoadmap(ctx context.Context) error {
	if err := g.RequireProject(); err != nil {
		return err
	}
	return g.RunCommand(ctx, "/gsd:create-roadmap")
}

// MapCodebase runs /gsd:map-codebase
func (g *GSD) MapCodebase(ctx context.Context) error {
	return g.RunCommand(ctx, "/gsd:map-codebase")
}

// ResearchPhase runs /gsd:research-phase N (requires ROADMAP.md)
func (g *GSD) ResearchPhase(ctx context.Context, phase int) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:research-phase %d", phase))
}

// DiscussPhase runs /gsd:discuss-phase N (requires ROADMAP.md)
func (g *GSD) DiscussPhase(ctx context.Context, phase int) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:discuss-phase %d", phase))
}

// PlanPhase runs /gsd:plan-phase N (requires ROADMAP.md)
func (g *GSD) PlanPhase(ctx context.Context, phase int) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:plan-phase %d", phase))
}

// AddPhase runs /gsd:add-phase "description" (requires ROADMAP.md)
func (g *GSD) AddPhase(ctx context.Context, description string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:add-phase \"%s\"", description))
}

// InsertPhase runs /gsd:insert-phase N "description" (requires ROADMAP.md)
func (g *GSD) InsertPhase(ctx context.Context, afterPhase int, description string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:insert-phase %d \"%s\"", afterPhase, description))
}

// RemovePhase runs /gsd:remove-phase N (requires ROADMAP.md)
func (g *GSD) RemovePhase(ctx context.Context, phase int) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:remove-phase %d", phase))
}

// Progress runs /gsd:progress
func (g *GSD) Progress(ctx context.Context) error {
	return g.RunCommand(ctx, "/gsd:progress")
}

func claudeNotFoundError() error {
	return fmt.Errorf(`claude not found in PATH

To fix, add to your ~/.zshrc or ~/.bashrc:
  export PATH="$HOME/.claude/local:$PATH"

Then restart your terminal, or run:
  source ~/.zshrc`)
}
