package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/daydemir/ralph/internal/prompts"
	"github.com/daydemir/ralph/internal/types"
	"github.com/daydemir/ralph/internal/utils"
)

// Planner handles all planning operations using internal prompts
type Planner struct {
	ClaudeBinary string
	WorkDir      string
}

// NewPlanner creates a new Planner instance
func NewPlanner(claudeBinary, workDir string) *Planner {
	if claudeBinary == "" {
		claudeBinary = "claude"
	}
	resolved := utils.ResolveBinaryPath(claudeBinary)
	return &Planner{
		ClaudeBinary: resolved,
		WorkDir:      workDir,
	}
}

// PlanningDir returns the path to .planning directory
func (p *Planner) PlanningDir() string {
	return filepath.Join(p.WorkDir, ".planning")
}

// HasProject checks if project.json exists
func (p *Planner) HasProject() bool {
	_, err := os.Stat(filepath.Join(p.PlanningDir(), "project.json"))
	return err == nil
}

// HasRoadmap checks if roadmap.json exists
func (p *Planner) HasRoadmap() bool {
	_, err := os.Stat(filepath.Join(p.PlanningDir(), "roadmap.json"))
	return err == nil
}

// HasState checks if state.json exists
func (p *Planner) HasState() bool {
	_, err := os.Stat(filepath.Join(p.PlanningDir(), "state.json"))
	return err == nil
}

// HasCodebaseMaps checks if codebase analysis exists
func (p *Planner) HasCodebaseMaps() bool {
	codebaseDir := filepath.Join(p.PlanningDir(), "codebase")
	if _, err := os.Stat(codebaseDir); os.IsNotExist(err) {
		return false
	}
	files, _ := os.ReadDir(codebaseDir)
	return len(files) > 0
}

// RequireProject returns an error if project.json doesn't exist
func (p *Planner) RequireProject() error {
	if !p.HasProject() {
		return fmt.Errorf(`no project.json found

Ralph requires proper planning before execution.
Run 'ralph discuss' first to create your project.`)
	}
	return nil
}

// RequireRoadmap returns an error if roadmap.json doesn't exist
func (p *Planner) RequireRoadmap() error {
	if err := p.RequireProject(); err != nil {
		return err
	}
	if !p.HasRoadmap() {
		return fmt.Errorf(`no roadmap found

You need a roadmap before planning individual phases.
Run 'ralph discuss' to create your roadmap.`)
	}
	return nil
}

// RunWithPrompt executes Claude with a given prompt
func (p *Planner) RunWithPrompt(ctx context.Context, prompt string) error {
	if _, err := exec.LookPath(p.ClaudeBinary); err != nil {
		return utils.ClaudeNotFoundError()
	}

	args := []string{"--dangerously-skip-permissions", "-p", prompt}

	cmd := exec.CommandContext(ctx, p.ClaudeBinary, args...)
	cmd.Dir = p.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return utils.ClaudeNotFoundError()
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// NewProject initializes a new project with project.json
func (p *Planner) NewProject(ctx context.Context) error {
	// Load the project initialization workflow
	prompt := `You are helping initialize a new project.

Your task:
1. Ask the user about their project (what they're building, tech stack, goals)
2. Create .planning/project.json with:
   - Project name and description
   - Tech stack and key dependencies
   - Goals and success criteria
   - Key constraints
3. Create .planning/state.json with initial state

Be conversational but efficient. Gather essential information, then create the files.

Start by asking what the user wants to build.`

	return p.RunWithPrompt(ctx, prompt)
}

// CreateRoadmap creates a roadmap from project.json
func (p *Planner) CreateRoadmap(ctx context.Context) error {
	if err := p.RequireProject(); err != nil {
		return err
	}

	// Read project.json for context
	projectPath := filepath.Join(p.PlanningDir(), "project.json")
	projectContent, err := os.ReadFile(projectPath)
	if err != nil {
		return fmt.Errorf("cannot read project.json: %w", err)
	}

	prompt := fmt.Sprintf(`You are creating a roadmap for this project.

## Project Context
%s

## Task
Create a roadmap.json file in .planning/ that breaks the project into phases.

Use this JSON structure:
{
  "projectName": "Project Name",
  "version": "1.0",
  "phases": [
    {
      "number": 1,
      "name": "foundation",
      "description": "Set up project foundation",
      "status": "pending",
      "plans": []
    }
  ],
  "createdAt": "2026-01-16T...",
  "updatedAt": "2026-01-16T..."
}

Guidelines:
- 3-7 phases typically
- Each phase should be independently valuable
- Order by dependency (foundation first)
- Keep descriptions concise but specific

Write the roadmap.json file now.`, string(projectContent))

	return p.RunWithPrompt(ctx, prompt)
}

// PlanPhase creates plans for a phase using the internal planner agent
func (p *Planner) PlanPhase(ctx context.Context, phase string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	// Load planner agent prompt
	plannerPrompt, err := prompts.GetAgent("planner")
	if err != nil {
		return fmt.Errorf("cannot load planner agent: %w", err)
	}

	// Read roadmap for context
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")
	roadmapContent, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	// Read project.json for context
	projectPath := filepath.Join(p.PlanningDir(), "project.json")
	projectContent, err := os.ReadFile(projectPath)
	if err != nil {
		projectContent = []byte("") // Optional
	}

	prompt := fmt.Sprintf(`%s

## Project Context
%s

## Roadmap
%s

## Task
Create plans for Phase %s.

1. Read the phase description from the roadmap
2. Apply goal-backward methodology to derive requirements
3. Break into 2-3 task plans
4. Write JSON plan files to .planning/phases/{phase-dir}/

Remember:
- Each plan has 2-3 tasks maximum
- Tasks need: files, action, verify, done
- Plans target ~50%% context usage

Begin planning.`, plannerPrompt, string(projectContent), string(roadmapContent), phase)

	if err := p.RunWithPrompt(ctx, prompt); err != nil {
		return err
	}

	// Sync roadmap after planning
	return p.SyncRoadmap(phase)
}

// DiscussPhase gathers context for a phase before planning
func (p *Planner) DiscussPhase(ctx context.Context, phase string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	// Read roadmap for context
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")
	roadmapContent, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	prompt := fmt.Sprintf(`You are gathering context for Phase %s before detailed planning.

## Roadmap
%s

## Task
Have a focused discussion to understand:
1. What the user envisions for this phase
2. Any specific requirements or constraints
3. Technology preferences
4. Must-have features vs nice-to-haves

Be conversational but efficient. Ask 3-5 clarifying questions, then summarize what you learned.

Create .planning/phases/{phase-dir}/{phase}-context.json with the discussion summary.

Begin the discussion.`, phase, string(roadmapContent))

	return p.RunWithPrompt(ctx, prompt)
}

// ResearchPhase conducts research before planning
func (p *Planner) ResearchPhase(ctx context.Context, phase string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	// Load researcher agent prompt
	researcherPrompt, err := prompts.GetAgent("researcher")
	if err != nil {
		return fmt.Errorf("cannot load researcher agent: %w", err)
	}

	// Read roadmap for phase description
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")
	roadmapContent, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	prompt := fmt.Sprintf(`%s

## Roadmap
%s

## Task
Research Phase %s. Determine the discovery level and conduct appropriate research.

If Level 2+, create .planning/phases/{phase-dir}/{phase}-research.json with findings.

Begin research.`, researcherPrompt, string(roadmapContent), phase)

	return p.RunWithPrompt(ctx, prompt)
}

// ReviewPlans reviews plans before execution
func (p *Planner) ReviewPlans(ctx context.Context, phase string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	// Load plan-checker agent
	checkerPrompt, err := prompts.GetAgent("plan-checker")
	if err != nil {
		return fmt.Errorf("cannot load plan-checker agent: %w", err)
	}

	prompt := fmt.Sprintf(`%s

## Task
Review all plans in Phase %s.

1. Load and analyze all plan JSON files
2. Check all six verification dimensions
3. Present findings to user
4. Allow user to request changes

If issues found, explain them clearly. If plans look good, summarize and confirm ready for execution.

Begin review.`, checkerPrompt, phase)

	return p.RunWithPrompt(ctx, prompt)
}

// AddPhase adds a new phase to the roadmap
func (p *Planner) AddPhase(ctx context.Context, description string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	// Read current roadmap
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")
	data, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	var roadmap types.Roadmap
	if err := json.Unmarshal(data, &roadmap); err != nil {
		return fmt.Errorf("cannot parse roadmap.json: %w", err)
	}

	// Find next phase number
	nextNumber := 1
	for _, phase := range roadmap.Phases {
		if phase.Number >= nextNumber {
			nextNumber = phase.Number + 1
		}
	}

	// Create new phase
	newPhase := types.Phase{
		Number: nextNumber,
		Name:   fmt.Sprintf("phase-%d", nextNumber),
		Goal:   description,
		Status: types.StatusPending,
		Plans:  []string{},
	}

	roadmap.Phases = append(roadmap.Phases, newPhase)

	// Write updated roadmap
	newData, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal roadmap: %w", err)
	}

	if err := os.WriteFile(roadmapPath, newData, 0644); err != nil {
		return fmt.Errorf("cannot write roadmap: %w", err)
	}

	fmt.Printf("Added Phase %d: %s\n", nextNumber, description)
	return nil
}

// InsertPhase inserts a new phase after a specific phase
func (p *Planner) InsertPhase(ctx context.Context, afterPhase string, description string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	afterNum, err := strconv.Atoi(afterPhase)
	if err != nil {
		return fmt.Errorf("invalid phase number: %s", afterPhase)
	}

	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("phase description cannot be empty")
	}

	// Read current roadmap
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")
	data, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	var roadmap types.Roadmap
	if err := json.Unmarshal(data, &roadmap); err != nil {
		return fmt.Errorf("cannot parse roadmap.json: %w", err)
	}

	// Find insertion point
	insertIdx := -1
	for i, phase := range roadmap.Phases {
		if phase.Number == afterNum {
			insertIdx = i + 1
			break
		}
	}

	if insertIdx == -1 {
		return fmt.Errorf("phase %d not found", afterNum)
	}

	// Renumber subsequent phases
	for i := insertIdx; i < len(roadmap.Phases); i++ {
		roadmap.Phases[i].Number++
	}

	// Create new phase
	newPhase := types.Phase{
		Number: afterNum + 1,
		Name:   fmt.Sprintf("phase-%d", afterNum+1),
		Goal:   description,
		Status: types.StatusPending,
		Plans:  []string{},
	}

	// Insert
	roadmap.Phases = append(roadmap.Phases[:insertIdx], append([]types.Phase{newPhase}, roadmap.Phases[insertIdx:]...)...)

	// Write updated roadmap
	newData, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal roadmap: %w", err)
	}

	if err := os.WriteFile(roadmapPath, newData, 0644); err != nil {
		return fmt.Errorf("cannot write roadmap: %w", err)
	}

	fmt.Printf("Inserted Phase %d: %s\n", afterNum+1, description)
	return nil
}

// RemovePhase removes a phase from the roadmap
func (p *Planner) RemovePhase(ctx context.Context, phase string) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	phaseNum, err := strconv.Atoi(phase)
	if err != nil {
		return fmt.Errorf("invalid phase number: %s", phase)
	}

	// Read current roadmap
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")
	data, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	var roadmap types.Roadmap
	if err := json.Unmarshal(data, &roadmap); err != nil {
		return fmt.Errorf("cannot parse roadmap.json: %w", err)
	}

	// Find and remove
	removeIdx := -1
	for i, p := range roadmap.Phases {
		if p.Number == phaseNum {
			removeIdx = i
			break
		}
	}

	if removeIdx == -1 {
		return fmt.Errorf("phase %d not found", phaseNum)
	}

	// Remove
	roadmap.Phases = append(roadmap.Phases[:removeIdx], roadmap.Phases[removeIdx+1:]...)

	// Renumber subsequent phases
	for i := removeIdx; i < len(roadmap.Phases); i++ {
		roadmap.Phases[i].Number = removeIdx + 1 + i - removeIdx
	}

	// Write updated roadmap
	newData, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal roadmap: %w", err)
	}

	if err := os.WriteFile(roadmapPath, newData, 0644); err != nil {
		return fmt.Errorf("cannot write roadmap: %w", err)
	}

	fmt.Printf("Removed Phase %d\n", phaseNum)
	return nil
}

// Progress shows current project progress
func (p *Planner) Progress(ctx context.Context) error {
	prompt := `Show the current project progress.

Read:
- .planning/roadmap.json
- .planning/state.json

Display:
1. Overall progress (phases complete / total)
2. Current phase status
3. Plans in current phase (complete / total)
4. What's next

Keep it concise and actionable.`

	return p.RunWithPrompt(ctx, prompt)
}

// UpdateRoadmap allows conversational updates to the roadmap
func (p *Planner) UpdateRoadmap(ctx context.Context) error {
	if err := p.RequireRoadmap(); err != nil {
		return err
	}

	prompt := `You are helping update the roadmap based on user feedback.

Read .planning/roadmap.json and discuss what changes the user wants.
Allow:
- Reordering phases
- Updating descriptions
- Adding/removing phases
- Splitting or merging phases

Be conversational. After understanding the changes, update roadmap.json.`

	return p.RunWithPrompt(ctx, prompt)
}

// MapCodebase analyzes the codebase
func (p *Planner) MapCodebase(ctx context.Context) error {
	prompt := `Analyze this codebase and create documentation in .planning/codebase/

Create these files:
1. STACK.md - Technologies, frameworks, key dependencies
2. STRUCTURE.md - Directory layout, key files
3. CONVENTIONS.md - Code style, patterns used
4. ARCHITECTURE.md - System design, data flow

Be thorough but concise. Focus on what matters for future planning.`

	return p.RunWithPrompt(ctx, prompt)
}

// SyncRoadmap updates roadmap.json to reflect actual plan files created
func (p *Planner) SyncRoadmap(phase string) error {
	roadmapPath := filepath.Join(p.PlanningDir(), "roadmap.json")

	// Parse phase string to get major phase number
	majorPhase := 0
	if parts := strings.Split(phase, "."); len(parts) >= 1 {
		majorPhase, _ = strconv.Atoi(parts[0])
	}

	// Read current roadmap.json
	data, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read roadmap.json: %w", err)
	}

	var roadmap types.Roadmap
	if err := json.Unmarshal(data, &roadmap); err != nil {
		return fmt.Errorf("cannot parse roadmap.json: %w", err)
	}

	// Get actual plan files for this phase
	phasesDir := filepath.Join(p.PlanningDir(), "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return fmt.Errorf("cannot read phases directory %s: %w", phasesDir, err)
	}

	// Find the phase directory
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

	// Extract plan IDs from JSON files
	var planIDs []string
	for _, entry := range planEntries {
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && !strings.Contains(name, "SUMMARY") {
			planID := strings.TrimSuffix(name, ".json")
			planIDs = append(planIDs, planID)
		}
	}

	if len(planIDs) == 0 {
		return nil // No plans to sync
	}

	// Update the matching phase in roadmap
	for i := range roadmap.Phases {
		if roadmap.Phases[i].Number == majorPhase {
			roadmap.Phases[i].Plans = planIDs
			break
		}
	}

	// Write updated roadmap.json
	newData, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal roadmap: %w", err)
	}

	tempPath := roadmapPath + ".tmp"
	if err := os.WriteFile(tempPath, newData, 0644); err != nil {
		return fmt.Errorf("cannot write temp roadmap file: %w", err)
	}

	if err := os.Rename(tempPath, roadmapPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("cannot rename temp roadmap file: %w", err)
	}

	return nil
}
