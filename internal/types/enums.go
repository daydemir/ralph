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
