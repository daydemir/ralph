package types

// TaskType represents the type of a task in a plan
type TaskType string

const (
	// TaskTypeAuto is executed fully autonomously by Claude
	TaskTypeAuto TaskType = "auto"
	// TaskTypeManual requires human interaction (kicks up interactive Claude)
	TaskTypeManual TaskType = "manual"
)

// IsValid checks if a task type is valid
func (t TaskType) IsValid() bool {
	for _, valid := range AllTaskTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// AllTaskTypes returns all valid task type values
func AllTaskTypes() []TaskType {
	return []TaskType{TaskTypeAuto, TaskTypeManual}
}

// String returns the string representation of the task type
func (t TaskType) String() string {
	return string(t)
}

// Status represents the execution status of phases, plans, and tasks
// This is a unified enum used across all state tracking (DRY principle)
type Status string

const (
	// StatusPending indicates work has not started
	StatusPending Status = "pending"
	// StatusInProgress indicates work is currently executing
	StatusInProgress Status = "in_progress"
	// StatusComplete indicates work has successfully finished
	StatusComplete Status = "complete"
	// StatusFailed indicates work has failed
	StatusFailed Status = "failed"
)

// IsValid checks if a status value is valid
func (s Status) IsValid() bool {
	for _, valid := range AllStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}

// AllStatuses returns all valid status values
func AllStatuses() []Status {
	return []Status{StatusPending, StatusInProgress, StatusComplete, StatusFailed}
}

// String returns the string representation of the status
func (s Status) String() string {
	return string(s)
}

// ObservationType represents the type of observation
type ObservationType string

const (
	ObsTypeBug                ObservationType = "bug"
	ObsTypeStub               ObservationType = "stub"
	ObsTypeAPIIssue           ObservationType = "api-issue"
	ObsTypeInsight            ObservationType = "insight"
	ObsTypeBlocker            ObservationType = "blocker"
	ObsTypeTechnicalDebt      ObservationType = "technical-debt"
	ObsTypeAssumption         ObservationType = "assumption"
	ObsTypeScopeCreep         ObservationType = "scope-creep"
	ObsTypeDependency         ObservationType = "dependency"
	ObsTypeQuestionable       ObservationType = "questionable"
	ObsTypeAlreadyComplete    ObservationType = "already-complete"
	ObsTypeManualDeferred     ObservationType = "manual-checkpoint-deferred"
	ObsTypeToolingFriction    ObservationType = "tooling-friction"
	ObsTypeTestFailed         ObservationType = "test-failed"
	ObsTypeTestInfrastructure ObservationType = "test-infrastructure"
)

// IsValid checks if an observation type is valid
func (o ObservationType) IsValid() bool {
	for _, valid := range AllObservationTypes() {
		if o == valid {
			return true
		}
	}
	return false
}

// AllObservationTypes returns all valid observation type values
func AllObservationTypes() []ObservationType {
	return []ObservationType{
		ObsTypeBug, ObsTypeStub, ObsTypeAPIIssue, ObsTypeInsight,
		ObsTypeBlocker, ObsTypeTechnicalDebt, ObsTypeAssumption,
		ObsTypeScopeCreep, ObsTypeDependency, ObsTypeQuestionable,
		ObsTypeAlreadyComplete, ObsTypeManualDeferred, ObsTypeToolingFriction,
		ObsTypeTestFailed, ObsTypeTestInfrastructure,
	}
}

// String returns the string representation of the observation type
func (o ObservationType) String() string {
	return string(o)
}

// Severity represents observation severity
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// IsValid checks if a severity value is valid
func (s Severity) IsValid() bool {
	for _, valid := range AllSeverities() {
		if s == valid {
			return true
		}
	}
	return false
}

// AllSeverities returns all valid severity values
func AllSeverities() []Severity {
	return []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
}

// String returns the string representation of the severity
func (s Severity) String() string {
	return string(s)
}

// ObservationAction represents what action is needed
type ObservationAction string

const (
	ActionNeedsFix            ObservationAction = "needs-fix"
	ActionNeedsImplementation ObservationAction = "needs-implementation"
	ActionNeedsPlan           ObservationAction = "needs-plan"
	ActionNeedsInvestigation  ObservationAction = "needs-investigation"
	ActionNeedsDocumentation  ObservationAction = "needs-documentation"
	ActionNeedsHumanVerify    ObservationAction = "needs-human-verify"
	ActionNone                ObservationAction = "none"
)

// IsValid checks if an observation action is valid
func (a ObservationAction) IsValid() bool {
	for _, valid := range AllObservationActions() {
		if a == valid {
			return true
		}
	}
	return false
}

// AllObservationActions returns all valid observation action values
func AllObservationActions() []ObservationAction {
	return []ObservationAction{
		ActionNeedsFix, ActionNeedsImplementation, ActionNeedsPlan,
		ActionNeedsInvestigation, ActionNeedsDocumentation,
		ActionNeedsHumanVerify, ActionNone,
	}
}

// String returns the string representation of the observation action
func (a ObservationAction) String() string {
	return string(a)
}
