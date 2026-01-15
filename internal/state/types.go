package state

import "fmt"

// PlanType represents the execution type of a plan
type PlanType string

const (
	PlanTypeExecute   PlanType = "execute"
	PlanTypeManual    PlanType = "manual"
	PlanTypeDecisions PlanType = "decisions"
)

// ValidPlanTypes is the exhaustive list of allowed types
var ValidPlanTypes = []PlanType{PlanTypeExecute, PlanTypeManual, PlanTypeDecisions}

// IsValid checks if a plan type is valid
func (t PlanType) IsValid() bool {
	for _, valid := range ValidPlanTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// PlanFrontmatter represents the YAML frontmatter in a PLAN.md file
type PlanFrontmatter struct {
	Phase  string   `yaml:"phase"`
	Plan   string   `yaml:"plan"`
	Type   PlanType `yaml:"type"`
	Status string   `yaml:"status"`
}

// Validate ensures frontmatter conforms to strict requirements
func (fm *PlanFrontmatter) Validate() error {
	if fm.Plan == "" {
		return fmt.Errorf("frontmatter: plan number is required")
	}
	if fm.Type == "" {
		fm.Type = PlanTypeExecute // Default
	}
	if !fm.Type.IsValid() {
		return fmt.Errorf("frontmatter: invalid type %q, must be one of: execute, manual, decisions", fm.Type)
	}
	return nil
}
