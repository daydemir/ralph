package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Progress tracks execution progress across PRDs
type Progress struct {
	SchemaVersion    string            `json:"schema_version"`
	CodebasePatterns []CodebasePattern `json:"codebase_patterns"`
	Observations     []ProgressObs     `json:"observations"`
	Learnings        []Learning        `json:"learnings"`
	PRDCompletions   []PRDCompletion   `json:"prd_completions"`
}

// CodebasePattern represents a discovered pattern in the codebase
type CodebasePattern struct {
	Pattern      string    `json:"pattern"`
	DiscoveredAt time.Time `json:"discovered_at,omitempty"`
	DiscoveredIn string    `json:"discovered_in,omitempty"`
}

// ProgressObs represents an observation made during execution
type ProgressObs struct {
	Timestamp   time.Time `json:"timestamp,omitempty"`
	PRDID       string    `json:"prd_id,omitempty"`
	Observation string    `json:"observation"`
	Category    string    `json:"category,omitempty"`
}

// Learning represents something learned during execution
type Learning struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	PRDID     string    `json:"prd_id,omitempty"`
	Learning  string    `json:"learning"`
	AppliesTo []string  `json:"applies_to,omitempty"`
}

// PRDCompletion records a completed PRD
type PRDCompletion struct {
	PRDID              string    `json:"prd_id"`
	CompletedAt        time.Time `json:"completed_at"`
	Summary            string    `json:"summary,omitempty"`
	FilesChanged       []string  `json:"files_changed,omitempty"`
	VerificationPassed bool      `json:"verification_passed"`
}

// NewProgress creates a new Progress with defaults
func NewProgress() *Progress {
	return &Progress{
		SchemaVersion:    "1.0",
		CodebasePatterns: []CodebasePattern{},
		Observations:     []ProgressObs{},
		Learnings:        []Learning{},
		PRDCompletions:   []PRDCompletion{},
	}
}

// Validate ensures the progress is valid
func (p *Progress) Validate() error {
	if p.SchemaVersion == "" {
		return fmt.Errorf("progress.schema_version: field is required")
	}
	return nil
}

// LoadProgress reads and parses a progress JSON file
func LoadProgress(path string) (*Progress, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return &progress, nil
}

// Save writes the progress to disk
func (p *Progress) Save(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// AddObservation adds an observation to the progress
func (p *Progress) AddObservation(prdID, observation, category string) {
	p.Observations = append(p.Observations, ProgressObs{
		Timestamp:   time.Now(),
		PRDID:       prdID,
		Observation: observation,
		Category:    category,
	})
}

// AddLearning adds a learning to the progress
func (p *Progress) AddLearning(prdID, learning string, appliesTo []string) {
	p.Learnings = append(p.Learnings, Learning{
		Timestamp: time.Now(),
		PRDID:     prdID,
		Learning:  learning,
		AppliesTo: appliesTo,
	})
}

// AddPattern adds a codebase pattern to the progress
func (p *Progress) AddPattern(pattern, discoveredIn string) {
	p.CodebasePatterns = append(p.CodebasePatterns, CodebasePattern{
		Pattern:      pattern,
		DiscoveredAt: time.Now(),
		DiscoveredIn: discoveredIn,
	})
}

// RecordCompletion records a PRD completion
func (p *Progress) RecordCompletion(prdID, summary string, filesChanged []string, verificationPassed bool) {
	p.PRDCompletions = append(p.PRDCompletions, PRDCompletion{
		PRDID:              prdID,
		CompletedAt:        time.Now(),
		Summary:            summary,
		FilesChanged:       filesChanged,
		VerificationPassed: verificationPassed,
	})
}
