package prd

import (
	"fmt"

	"github.com/daydemir/ralph/internal/types"
)

// Validate performs simple validation for saves
func (p *PRD) Validate() error {
	if p.Version == "" {
		return fmt.Errorf("prd.version: field is required")
	}
	if p.ID == "" {
		return fmt.Errorf("prd.id: field is required")
	}
	if p.Title == "" {
		return fmt.Errorf("prd.title: field is required")
	}
	if p.Status == "" {
		p.Status = types.StatusPending
	}
	if !p.Status.IsValid() {
		return fmt.Errorf("prd.status: invalid value %q, must be one of: %v", p.Status, types.AllStatuses())
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("prd.created_at: field is required")
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("prd.updated_at: field is required")
	}
	return nil
}

// ValidateWithDetails performs rich validation for self-healing
func (p *PRD) ValidateWithDetails() *types.ValidationErrors {
	errs := &types.ValidationErrors{}

	if p.Version == "" {
		errs.Add("version", "non-empty string", "", "Provide schema version like \"1.0\"")
	}
	if p.ID == "" {
		errs.Add("id", "non-empty string", "", "Provide PRD ID like \"auth-login-a1b2\"")
	}
	if p.Title == "" {
		errs.Add("title", "non-empty string", "", "Provide a descriptive title")
	}
	if p.Status == "" {
		p.Status = types.StatusPending
	}
	if !p.Status.IsValid() {
		errs.Add(
			"status",
			fmt.Sprintf("one of: %v", types.AllStatuses()),
			p.Status,
			fmt.Sprintf("Change status to one of the valid values (not %q)", p.Status),
		)
	}
	if p.Description == "" {
		errs.Add("description", "non-empty string", "", "Provide a description of what this PRD accomplishes")
	}
	if len(p.AcceptanceCriteria) == 0 {
		errs.Add("acceptance_criteria", "array with at least one criterion", []string{}, "Add at least one acceptance criterion")
	}
	if len(p.Steps) == 0 {
		errs.Add("steps", "array with at least one step", []string{}, "Add at least one implementation step")
	}
	if p.CreatedAt.IsZero() {
		errs.Add("created_at", "ISO 8601 timestamp", nil, "Provide created_at timestamp")
	}
	if p.UpdatedAt.IsZero() {
		errs.Add("updated_at", "ISO 8601 timestamp", nil, "Provide updated_at timestamp")
	}
	if p.MaxIterations <= 0 {
		errs.Add("max_iterations", "positive integer", p.MaxIterations, "Set max_iterations to a positive value (e.g., 3)")
	}

	return errs
}
