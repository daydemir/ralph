package state

import "github.com/daydemir/ralph/internal/types"

// Plan type constants for backward compatibility with executor
const (
	PlanTypeExecute   = "execute"
	PlanTypeManual    = "manual"
	PlanTypeDecisions = "decisions"
)

// Phase wraps types.Phase with additional runtime fields for executor compatibility
// This is a temporary bridge until executor is refactored to work with pure types.Phase
type Phase struct {
	Number      int
	Name        string
	Path        string // Derived from phase directory
	Plans       []Plan
	IsCompleted bool
}

// Plan wraps types.Plan with additional runtime fields for executor compatibility
// This is a temporary bridge until executor is refactored to work with pure types.Plan
type Plan struct {
	Number      string // String to support decimal plan numbers like "5.1"
	Name        string
	Path        string // Derived from plan file path
	Type        string // "execute", "manual", "decisions"
	Status      string
	IsCompleted bool // Has corresponding SUMMARY.md
}

// IsManual returns true if the plan requires manual execution
func (p *Plan) IsManual() bool {
	return p.Type == "manual"
}

// FromTypesPhase converts types.Phase to state.Phase wrapper with path
func FromTypesPhase(tp *types.Phase, phasePath string) *Phase {
	if tp == nil {
		return nil
	}
	return &Phase{
		Number: tp.Number,
		Name:   tp.Name,
		Path:   phasePath,
		Plans:  []Plan{}, // Plans populated separately
	}
}

// FromTypesPlan converts types.Plan to state.Plan wrapper with path
func FromTypesPlan(tp *types.Plan, planPath string) *Plan {
	if tp == nil {
		return nil
	}
	return &Plan{
		Number: tp.PlanNumber,
		Path:   planPath,
		Type:   string(tp.Status), // Note: this is a simplification, may need adjustment
		Status: string(tp.Status),
	}
}
