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

	// Runtime fields (not serialized to JSON)
	Path        string `json:"-"` // Filesystem path to phase directory
	IsCompleted bool   `json:"-"` // Whether all plans in phase are complete
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
	Phase              string     `json:"phase"`                         // Phase ID like "01-critical-bug-fixes"
	PlanNumber         string     `json:"plan_number"`                   // Plan number like "01" (supports decimals like "01.1")
	Status             Status     `json:"status"`                        // Uses unified Status enum
	Objective          string     `json:"objective"`                     // Plan objective
	Tasks              []Task     `json:"tasks"`                         // List of tasks
	Verification       []string   `json:"verification"`                  // Verification commands
	ValidationCommands []string   `json:"validation_commands,omitempty"` // Commands that MUST pass before plan can be marked complete
	CreatedAt          time.Time  `json:"created_at"`                    // When plan was created
	CompletedAt        *time.Time `json:"completed_at,omitempty"`        // When plan was completed (optional)

	// Runtime fields (not serialized to JSON)
	Path        string `json:"-"` // Filesystem path to plan JSON file
	Name        string `json:"-"` // Derived from objective (first sentence or 80 chars)
	IsCompleted bool   `json:"-"` // Status == StatusComplete
}

// PlanType constants for categorizing plans
const (
	PlanTypeExecute   = "execute"
	PlanTypeManual    = "manual"
	PlanTypeDecisions = "decisions"
)

// GetType returns the plan type based on tasks
// If any task is manual, the plan is manual; otherwise execute
func (p *Plan) GetType() string {
	for _, task := range p.Tasks {
		if task.Type == TaskTypeManual {
			return PlanTypeManual
		}
	}
	return PlanTypeExecute
}

// IsManual returns true if the plan contains any manual tasks
func (p *Plan) IsManual() bool {
	return p.GetType() == PlanTypeManual
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

// ValidateWithDetails performs detailed validation and returns structured errors
// This is used by the self-healing validation loop to provide precise error information
func (p *Plan) ValidateWithDetails() *ValidationErrors {
	errs := &ValidationErrors{}

	if p.Phase == "" {
		errs.Add(
			"phase",
			"non-empty string",
			"",
			"Provide phase ID like \"01-critical-bug-fixes\"",
		)
	}
	if p.PlanNumber == "" {
		errs.Add(
			"plan_number",
			"non-empty string",
			"",
			"Provide plan number like \"01\" or \"01.1\"",
		)
	}
	if p.Status == "" {
		p.Status = StatusPending // Default to pending
	}
	if !p.Status.IsValid() {
		errs.Add(
			"status",
			fmt.Sprintf("one of: %v", AllStatuses()),
			p.Status,
			fmt.Sprintf("Change status to one of the valid values (not %q)", p.Status),
		)
	}
	if p.Objective == "" {
		errs.Add(
			"objective",
			"non-empty string",
			"",
			"Provide plan objective describing what this plan accomplishes",
		)
	}
	if len(p.Tasks) == 0 {
		errs.Add(
			"tasks",
			"array with at least one task",
			[]string{},
			"Add at least one task to the plan",
		)
	}
	for i, task := range p.Tasks {
		taskErrs := task.ValidateWithDetails(fmt.Sprintf("tasks[%d]", i))
		if taskErrs.HasErrors() {
			errs.Errors = append(errs.Errors, taskErrs.Errors...)
		}
	}
	if p.CreatedAt.IsZero() {
		errs.Add(
			"created_at",
			"ISO 8601 timestamp",
			nil,
			"Provide created_at timestamp in ISO 8601 format",
		)
	}

	return errs
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
	// Done is required for all tasks
	if t.Done == "" {
		return fmt.Errorf("task.done: field is required")
	}
	// Verify is required for auto tasks (manual tasks may not have automated verification)
	if t.Type == TaskTypeAuto && t.Verify == "" {
		return fmt.Errorf("task.verify: field is required for auto tasks")
	}
	if t.Status == "" {
		t.Status = StatusPending // Default to pending
	}
	if !t.Status.IsValid() {
		return fmt.Errorf("task.status: invalid value %q, must be one of: %v", t.Status, AllStatuses())
	}
	return nil
}

// ValidateWithDetails performs detailed validation and returns structured errors
// This is used by the self-healing validation loop to provide precise error information
func (t *Task) ValidateWithDetails(fieldPrefix string) *ValidationErrors {
	errs := &ValidationErrors{}

	if t.ID == "" {
		errs.Add(
			fieldPrefix+".id",
			"non-empty string",
			"",
			"Provide a task ID like \"task-1\"",
		)
	}
	if t.Name == "" {
		errs.Add(
			fieldPrefix+".name",
			"non-empty string",
			"",
			"Provide a descriptive task name",
		)
	}
	if t.Type == "" {
		t.Type = TaskTypeAuto // Default to auto
	}
	if !t.Type.IsValid() {
		errs.Add(
			fieldPrefix+".type",
			fmt.Sprintf("one of: %v", AllTaskTypes()),
			t.Type,
			fmt.Sprintf("Change task type to \"auto\" or \"manual\" (not %q)", t.Type),
		)
	}
	if t.Action == "" {
		errs.Add(
			fieldPrefix+".action",
			"non-empty string",
			"",
			"Provide task action describing what to do",
		)
	}
	// Done is required for all tasks
	if t.Done == "" {
		errs.Add(
			fieldPrefix+".done",
			"non-empty string",
			"",
			"Provide acceptance criteria describing when this task is complete",
		)
	}
	// Verify is required for auto tasks (manual tasks may not have automated verification)
	if t.Type == TaskTypeAuto && t.Verify == "" {
		errs.Add(
			fieldPrefix+".verify",
			"non-empty string (required for auto tasks)",
			"",
			"Provide verification command or criteria for auto task",
		)
	}
	if t.Status == "" {
		t.Status = StatusPending // Default to pending
	}
	if !t.Status.IsValid() {
		errs.Add(
			fieldPrefix+".status",
			fmt.Sprintf("one of: %v", AllStatuses()),
			t.Status,
			fmt.Sprintf("Change status to one of the valid values (not %q)", t.Status),
		)
	}

	return errs
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

// Project represents the project definition (project.json)
type Project struct {
	Version     string    `json:"version"`     // Schema version "1.0"
	Name        string    `json:"name"`        // Project name
	Description string    `json:"description"` // Project description
	Goals       []string  `json:"goals"`       // Project goals
	TechStack   []string  `json:"tech_stack"`  // Technologies used
	Constraints []string  `json:"constraints"` // Key constraints
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate ensures the project is valid
func (p *Project) Validate() error {
	if p.Version == "" {
		return fmt.Errorf("project.version: field is required")
	}
	if p.Name == "" {
		return fmt.Errorf("project.name: field is required")
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("project.created_at: field is required")
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("project.updated_at: field is required")
	}
	return nil
}

// Summary represents a plan execution summary ({phase}-{plan}-summary.json)
type Summary struct {
	Version        string        `json:"version"`
	Phase          string        `json:"phase"`
	PlanNumber     string        `json:"plan_number"`
	OneLiner       string        `json:"one_liner"`       // Substantive summary
	TasksCompleted []TaskResult  `json:"tasks_completed"`
	KeyChanges     []string      `json:"key_changes"`
	FilesModified  []string      `json:"files_modified"`
	Deviations     []Deviation   `json:"deviations"`
	Observations   []Observation `json:"observations"`
	Duration       string        `json:"duration"`
	CreatedAt      time.Time     `json:"created_at"`
}

// TaskResult represents a completed task in the summary
type TaskResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Commit string `json:"commit"`
}

// Deviation represents a deviation from the plan
type Deviation struct {
	Rule   int      `json:"rule"`   // Rule 1-4
	Type   string   `json:"type"`   // bug, blocking, etc.
	Title  string   `json:"title"`
	Issue  string   `json:"issue"`
	Fix    string   `json:"fix"`
	Files  []string `json:"files"`
	Commit string   `json:"commit"`
}

// Observation represents a finding captured during execution
// Simplified: Running agents observe, analyzer infers severity and decides actions
type Observation struct {
	Type        string `json:"type"`                  // blocker, finding, or completion
	Title       string `json:"title"`                 // Short descriptive title
	Description string `json:"description,omitempty"` // What was noticed and agent's thoughts
	File        string `json:"file,omitempty"`        // Where (optional)
}

// Validate ensures the summary is valid
func (s *Summary) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("summary.version: field is required")
	}
	if s.Phase == "" {
		return fmt.Errorf("summary.phase: field is required")
	}
	if s.PlanNumber == "" {
		return fmt.Errorf("summary.plan_number: field is required")
	}
	if s.OneLiner == "" {
		return fmt.Errorf("summary.one_liner: field is required")
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("summary.created_at: field is required")
	}
	return nil
}

// Context represents phase discussion context ({phase}-context.json)
type Context struct {
	Version           string    `json:"version"`
	Phase             int       `json:"phase"`
	PhaseName         string    `json:"phase_name"`
	DiscussionSummary string    `json:"discussion_summary"`
	UserVision        []string  `json:"user_vision"`
	Requirements      []string  `json:"requirements"`
	Constraints       []string  `json:"constraints"`
	Preferences       []string  `json:"preferences"`
	MustHaves         []string  `json:"must_haves"`
	NiceToHaves       []string  `json:"nice_to_haves"`
	CreatedAt         time.Time `json:"created_at"`
}

// Validate ensures the context is valid
func (c *Context) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("context.version: field is required")
	}
	if c.Phase <= 0 {
		return fmt.Errorf("context.phase: must be positive")
	}
	if c.PhaseName == "" {
		return fmt.Errorf("context.phase_name: field is required")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("context.created_at: field is required")
	}
	return nil
}

// Research represents phase research findings ({phase}-research.json)
type Research struct {
	Version        string           `json:"version"`
	Phase          int              `json:"phase"`
	PhaseName      string           `json:"phase_name"`
	DiscoveryLevel int              `json:"discovery_level"` // 0-3
	Summary        string           `json:"summary"`
	Recommendation string           `json:"recommendation"`
	KeyFindings    []string         `json:"key_findings"`
	Risks          []Risk           `json:"risks"`
	Options        []ResearchOption `json:"options,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// Risk represents a risk identified during research
type Risk struct {
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"` // low, medium, high
	Impact      string `json:"impact"`     // low, medium, high
	Mitigation  string `json:"mitigation"`
}

// ResearchOption represents an option evaluated during research
type ResearchOption struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
	Score       int      `json:"score,omitempty"`
}

// Validate ensures the research is valid
func (r *Research) Validate() error {
	if r.Version == "" {
		return fmt.Errorf("research.version: field is required")
	}
	if r.Phase <= 0 {
		return fmt.Errorf("research.phase: must be positive")
	}
	if r.PhaseName == "" {
		return fmt.Errorf("research.phase_name: field is required")
	}
	if r.DiscoveryLevel < 0 || r.DiscoveryLevel > 3 {
		return fmt.Errorf("research.discovery_level: must be between 0 and 3")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("research.created_at: field is required")
	}
	return nil
}
