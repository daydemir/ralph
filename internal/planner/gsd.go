package planner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
func (g *GSD) ResearchPhase(ctx context.Context, phase string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:research-phase %s", phase))
}

// DiscussPhase runs /gsd:discuss-phase N (requires ROADMAP.md)
func (g *GSD) DiscussPhase(ctx context.Context, phase string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:discuss-phase %s", phase))
}

// PlanPhase runs /gsd:plan-phase N (requires ROADMAP.md)
// After planning, syncs ROADMAP.md with actual plan names
func (g *GSD) PlanPhase(ctx context.Context, phase string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	if err := g.RunCommand(ctx, fmt.Sprintf("/gsd:plan-phase %s", phase)); err != nil {
		return err
	}
	// Sync ROADMAP.md with actual plan files
	return g.SyncRoadmap(phase)
}

// AddPhase runs /gsd:add-phase "description" (requires ROADMAP.md)
func (g *GSD) AddPhase(ctx context.Context, description string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:add-phase \"%s\"", description))
}

// InsertPhase runs /gsd:insert-phase N "description" (requires ROADMAP.md)
func (g *GSD) InsertPhase(ctx context.Context, afterPhase string, description string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:insert-phase %s \"%s\"", afterPhase, description))
}

// RemovePhase runs /gsd:remove-phase N (requires ROADMAP.md)
func (g *GSD) RemovePhase(ctx context.Context, phase string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:remove-phase %s", phase))
}

// Progress runs /gsd:progress
func (g *GSD) Progress(ctx context.Context) error {
	return g.RunCommand(ctx, "/gsd:progress")
}

// ReviewPlans runs /gsd:review-plans N (requires plans to exist)
func (g *GSD) ReviewPlans(ctx context.Context, phase string) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, fmt.Sprintf("/gsd:review-plans %s", phase))
}

// UpdateRoadmap runs /gsd:update-roadmap (requires ROADMAP.md)
func (g *GSD) UpdateRoadmap(ctx context.Context) error {
	if err := g.RequireRoadmap(); err != nil {
		return err
	}
	return g.RunCommand(ctx, "/gsd:update-roadmap")
}

func claudeNotFoundError() error {
	return fmt.Errorf(`claude not found in PATH

To fix, add to your ~/.zshrc or ~/.bashrc:
  export PATH="$HOME/.claude/local:$PATH"

Then restart your terminal, or run:
  source ~/.zshrc`)
}

// SyncRoadmap updates ROADMAP.md to reflect actual plan files created
func (g *GSD) SyncRoadmap(phase string) error {
	roadmapPath := filepath.Join(g.PlanningDir(), "ROADMAP.md")

	// Parse phase string to get major phase number
	majorPhase := 0
	if parts := strings.Split(phase, "."); len(parts) >= 1 {
		majorPhase, _ = strconv.Atoi(parts[0])
	}

	// Read current ROADMAP.md
	content, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read ROADMAP.md: %w", err)
	}

	// Get actual plan files for this phase
	phasesDir := filepath.Join(g.PlanningDir(), "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return nil // No phases directory yet
	}

	// Find the phase directory (use major phase for directory lookup)
	var phaseDir string
	phasePrefix := fmt.Sprintf("%02d", majorPhase)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), phasePrefix) {
			phaseDir = filepath.Join(phasesDir, entry.Name())
			break
		}
	}

	if phaseDir == "" {
		return nil // Phase directory not found
	}

	// Read plan files
	planEntries, err := os.ReadDir(phaseDir)
	if err != nil {
		return nil
	}

	// Extract plan names from files
	type planInfo struct {
		number int
		title  string
	}
	var plans []planInfo

	for _, entry := range planEntries {
		name := entry.Name()
		if !strings.HasSuffix(name, "-PLAN.md") {
			continue
		}

		// Read plan file to get title
		planPath := filepath.Join(phaseDir, name)
		planContent, err := os.ReadFile(planPath)
		if err != nil {
			continue
		}

		// Extract plan number (e.g., "01-02-PLAN.md" -> 2)
		planNum := 0
		parts := strings.Split(strings.TrimSuffix(name, "-PLAN.md"), "-")
		if len(parts) >= 2 {
			planNum, _ = strconv.Atoi(parts[1])
		}

		// Extract title from first heading
		lines := strings.Split(string(planContent), "\n")
		title := "Plan"
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				// Remove common suffixes
				title = strings.TrimSuffix(title, " Plan")
				break
			}
		}

		plans = append(plans, planInfo{number: planNum, title: title})
	}

	if len(plans) == 0 {
		return nil // No plans to sync
	}

	// Sort by plan number
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].number < plans[j].number
	})

	// Update ROADMAP.md content
	lines := strings.Split(string(content), "\n")
	inPhase := false
	// Support both integer and decimal phase markers
	phaseMarker := fmt.Sprintf("## Phase %s", phase)
	nextMajorPhase := majorPhase + 1
	nextPhaseMarker := fmt.Sprintf("## Phase %d", nextMajorPhase)

	var newLines []string
	planIndex := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're entering the target phase
		if strings.HasPrefix(trimmed, phaseMarker) {
			inPhase = true
			newLines = append(newLines, line)
			continue
		}

		// Check if we're leaving the target phase
		if inPhase && (strings.HasPrefix(trimmed, nextPhaseMarker) || (strings.HasPrefix(trimmed, "## Phase") && !strings.HasPrefix(trimmed, phaseMarker))) {
			inPhase = false
		}

		// Replace plan lines within the phase
		if inPhase && strings.HasPrefix(trimmed, fmt.Sprintf("%02d-%02d:", majorPhase, 1)) {
			// Skip old plan lines - we'll add new ones
			continue
		}

		// If we're at the end of the phase section, add actual plans
		if inPhase && (trimmed == "" || strings.HasPrefix(trimmed, "## Phase")) && planIndex == 0 && len(plans) > 0 {
			// Insert actual plan entries
			for _, p := range plans {
				newLines = append(newLines, fmt.Sprintf("  %02d-%02d: %s", majorPhase, p.number, p.title))
			}
			planIndex = len(plans) // Mark as inserted
		}

		newLines = append(newLines, line)
	}

	// Write updated ROADMAP.md
	return os.WriteFile(roadmapPath, []byte(strings.Join(newLines, "\n")), 0644)
}
