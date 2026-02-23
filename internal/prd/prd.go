package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

// PRD represents a single product requirement document
type PRD struct {
	Version string       `json:"version"`
	ID      string       `json:"id"` // e.g., "auth-login-a1b2"
	Title   string       `json:"title"`
	Status  types.Status `json:"status"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Steps              []string `json:"steps"`

	DependsOn    []string `json:"depends_on,omitempty"`
	RelatedFiles []string `json:"related_files,omitempty"`

	Verification Verification `json:"verification"`

	CurrentIteration int `json:"current_iteration"`
	MaxIterations    int `json:"max_iterations"`

	Attempts []Attempt `json:"attempts,omitempty"`
}

// Verification holds the verification commands for a PRD
type Verification struct {
	Tests     []string `json:"tests,omitempty"`
	Build     []string `json:"build,omitempty"`
	TypeCheck []string `json:"type_check,omitempty"`
	Custom    []string `json:"custom,omitempty"`
}

// Attempt represents a single execution attempt of the PRD
type Attempt struct {
	Iteration      int       `json:"iteration"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at"`
	Outcome        string    `json:"outcome"` // partial, complete, blocked, no_progress
	StepsCompleted []string  `json:"steps_completed,omitempty"`
	StepsRemaining []string  `json:"steps_remaining,omitempty"`
	Blocker        string    `json:"blocker,omitempty"`
	Observations   []string  `json:"observations,omitempty"`
	EvidencePath   string    `json:"evidence_path,omitempty"`
}

// Load reads and parses a PRD JSON file
func Load(path string) (*PRD, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return &prd, nil
}

// Save writes the PRD to disk
func (p *PRD) Save(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRD: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// NewPRD creates a new PRD with defaults
func NewPRD(title string) *PRD {
	now := time.Now()
	return &PRD{
		Version:          "1.0",
		ID:               GenerateID(title),
		Title:            title,
		Status:           types.StatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
		CurrentIteration: 0,
		MaxIterations:    3,
	}
}
