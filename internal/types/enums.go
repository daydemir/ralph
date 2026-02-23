package types

// Status represents the execution status of PRDs
type Status string

const (
	// StatusPending indicates work has not started
	StatusPending Status = "pending"
	// StatusInProgress indicates work is currently executing
	StatusInProgress Status = "in_progress"
	// StatusPendingReview indicates work needs two-factor verification
	StatusPendingReview Status = "pending_review"
	// StatusComplete indicates work has successfully finished
	StatusComplete Status = "complete"
	// StatusBlocked indicates work is blocked and cannot proceed
	StatusBlocked Status = "blocked"
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
	return []Status{StatusPending, StatusInProgress, StatusPendingReview, StatusComplete, StatusBlocked}
}

// String returns the string representation of the status
func (s Status) String() string {
	return string(s)
}
