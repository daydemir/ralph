package types

import (
	"fmt"
	"time"
)

// Roadmap represents the full project roadmap (roadmap.json)
type Roadmap struct {
	Version     string  `json:"version"`      // Schema version
	ProjectName string  `json:"project_name"` // Project name
	Overview    string  `json:"overview"`     // Overview of the roadmap
	Phases      []Phase `json:"phases"`       // List of phases
}

// Validate ensures the roadmap is valid
func (r *Roadmap) Validate() error {
	if r.Version == "" {
		return fmt.Errorf("roadmap.version: field is required")
	}
	if r.ProjectName == "" {
		return fmt.Errorf("roadmap.project_name: field is required")
	}
	if len(r.Phases) == 0 {
		return fmt.Errorf("roadmap.phases: at least one phase is required")
	}
	for i, phase := range r.Phases {
		if err := phase.Validate(); err != nil {
			return fmt.Errorf("roadmap.phases[%d]: %w", i, err)
		}
	}
	return nil
}

// Phase represents a phase in the roadmap AND runtime state
type Phase struct {
	Number int      `json:"number"` // Phase number (e.g., 1, 2)
	Name   string   `json:"name"`   // Phase name
	Goal   string   `json:"goal"`   // Phase goal
	Status Status   `json:"status"` // Uses unified Status enum
	Plans  []string `json:"plans"`  // Plan IDs like "01-01", "01-02"
}

// Validate ensures the phase is valid
func (p *Phase) Validate() error {
	if p.Number <= 0 {
		return fmt.Errorf("phase.number: must be positive")
	}
	if p.Name == "" {
		return fmt.Errorf("phase.name: field is required")
	}
	if p.Goal == "" {
		return fmt.Errorf("phase.goal: field is required")
	}
	if p.Status == "" {
		p.Status = StatusPending // Default to pending
	}
	if !p.Status.IsValid() {
		return fmt.Errorf("phase.status: invalid value %q, must be one of: %v", p.Status, AllStatuses())
	}
	return nil
}

// Plan represents an individual plan (NN-MM.json)
type Plan struct {
	Phase        string     `json:"phase"`                  // Phase ID like "01-critical-bug-fixes"
	PlanNumber   string     `json:"plan_number"`            // Plan number like "01" (supports decimals like "01.1")
	Status       Status     `json:"status"`                 // Uses unified Status enum
	Objective    string     `json:"objective"`              // Plan objective
	Tasks        []Task     `json:"tasks"`                  // List of tasks
	Verification []string   `json:"verification"`           // Verification commands
	CreatedAt    time.Time  `json:"created_at"`             // When plan was created
	CompletedAt  *time.Time `json:"completed_at,omitempty"` // When plan was completed (optional)
}

// Validate ensures the plan is valid
func (p *Plan) Validate() error {
	if p.Phase == "" {
		return fmt.Errorf("plan.phase: field is required")
	}
	if p.PlanNumber == "" {
		return fmt.Errorf("plan.plan_number: field is required")
	}
	if p.Status == "" {
		p.Status = StatusPending // Default to pending
	}
	if !p.Status.IsValid() {
		return fmt.Errorf("plan.status: invalid value %q, must be one of: %v", p.Status, AllStatuses())
	}
	if p.Objective == "" {
		return fmt.Errorf("plan.objective: field is required")
	}
	if len(p.Tasks) == 0 {
		return fmt.Errorf("plan.tasks: at least one task is required")
	}
	for i, task := range p.Tasks {
		if err := task.Validate(); err != nil {
			return fmt.Errorf("plan.tasks[%d]: %w", i, err)
		}
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("plan.created_at: field is required")
	}
	return nil
}

// Task represents a task within a plan
type Task struct {
	ID          string     `json:"id"`                     // Task ID like "task-1"
	Name        string     `json:"name"`                   // Task name
	Type        TaskType   `json:"type"`                   // auto or manual
	Files       []string   `json:"files"`                  // Files to create/modify
	Action      string     `json:"action"`                 // What to do
	Verify      string     `json:"verify"`                 // How to verify
	Done        string     `json:"done"`                   // Acceptance criteria
	Status      Status     `json:"status"`                 // Uses unified Status enum
	CompletedAt *time.Time `json:"completed_at,omitempty"` // When task was completed (optional)
}

// Validate ensures the task is valid
func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task.id: field is required")
	}
	if t.Name == "" {
		return fmt.Errorf("task.name: field is required")
	}
	if t.Type == "" {
		t.Type = TaskTypeAuto // Default to auto
	}
	if !t.Type.IsValid() {
		return fmt.Errorf("task.type: invalid value %q, must be one of: %v", t.Type, AllTaskTypes())
	}
	if t.Action == "" {
		return fmt.Errorf("task.action: field is required")
	}
	if t.Status == "" {
		t.Status = StatusPending // Default to pending
	}
	if !t.Status.IsValid() {
		return fmt.Errorf("task.status: invalid value %q, must be one of: %v", t.Status, AllStatuses())
	}
	return nil
}

// ProjectState represents runtime state tracking (state.json)
type ProjectState struct {
	Version      string    `json:"version"`       // Schema version
	CurrentPhase int       `json:"current_phase"` // Current phase number
	LastUpdated  time.Time `json:"last_updated"`  // Last update timestamp
}

// Validate ensures the project state is valid
func (s *ProjectState) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("state.version: field is required")
	}
	if s.CurrentPhase < 0 {
		return fmt.Errorf("state.current_phase: must be non-negative")
	}
	if s.LastUpdated.IsZero() {
		return fmt.Errorf("state.last_updated: field is required")
	}
	return nil
}
