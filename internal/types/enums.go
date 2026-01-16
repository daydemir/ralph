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
// Radically simplified: Running agents just observe, analyzer decides actions
type ObservationType string

const (
	// ObsTypeBlocker indicates the agent can't continue without human intervention
	ObsTypeBlocker ObservationType = "blocker"
	// ObsTypeFinding indicates something interesting was noticed
	ObsTypeFinding ObservationType = "finding"
	// ObsTypeCompletion indicates work was already done or not needed
	ObsTypeCompletion ObservationType = "completion"
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
	return []ObservationType{ObsTypeBlocker, ObsTypeFinding, ObsTypeCompletion}
}

// String returns the string representation of the observation type
func (o ObservationType) String() string {
	return string(o)
}
