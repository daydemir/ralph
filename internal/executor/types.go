package executor

import (
	"regexp"
	"strings"
)

// TaskType represents the type of a task in plan XML
type TaskType string

const (
	// TaskTypeAuto is the default task type - executed automatically
	TaskTypeAuto TaskType = "auto"
	// TaskTypeManual requires human action
	TaskTypeManual TaskType = "manual"
	// TaskTypeCheckpoint is a human-action checkpoint that blocks execution
	TaskTypeCheckpoint TaskType = "checkpoint:human-action"
	// TaskTypeDecision is a decision point that requires human choice
	TaskTypeDecision TaskType = "checkpoint:decision"
)

// ValidTaskTypes is the exhaustive list of allowed task types
var ValidTaskTypes = []TaskType{
	TaskTypeAuto,
	TaskTypeManual,
	TaskTypeCheckpoint,
	TaskTypeDecision,
}

// IsValid checks if a task type is valid
func (t TaskType) IsValid() bool {
	for _, valid := range ValidTaskTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// RequiresHumanAction returns true if this task type requires human intervention
func (t TaskType) RequiresHumanAction() bool {
	return t == TaskTypeManual || t == TaskTypeCheckpoint
}

// IsDecisionPoint returns true if this task type is a decision checkpoint
func (t TaskType) IsDecisionPoint() bool {
	return t == TaskTypeDecision
}

// taskTypePattern is the compiled regex for extracting task type attributes
var taskTypePattern = regexp.MustCompile(`type="([^"]+)"`)

// ExtractTaskType parses a task XML element and returns its type.
// Returns TaskTypeAuto if no type attribute is found (default).
// Returns error if the type attribute is present but invalid.
func ExtractTaskType(taskXML string) (TaskType, error) {
	match := taskTypePattern.FindStringSubmatch(taskXML)
	if match == nil {
		return TaskTypeAuto, nil // default
	}
	taskType := TaskType(match[1])
	if !taskType.IsValid() {
		return "", &InvalidTaskTypeError{Type: match[1]}
	}
	return taskType, nil
}

// InvalidTaskTypeError is returned when an unknown task type is encountered
type InvalidTaskTypeError struct {
	Type string
}

func (e *InvalidTaskTypeError) Error() string {
	return "invalid task type: " + e.Type + " (valid: auto, manual, checkpoint:human-action, checkpoint:decision)"
}

// ValidatePlanTasks checks that all task types in a plan content are valid.
// Returns an error describing any invalid task types found.
func ValidatePlanTasks(planContent string) error {
	taskTypePattern := regexp.MustCompile(`<task\s+type="([^"]+)"`)
	matches := taskTypePattern.FindAllStringSubmatch(planContent, -1)

	var invalidTypes []string
	for _, match := range matches {
		if len(match) > 1 {
			taskType := TaskType(match[1])
			if !taskType.IsValid() {
				invalidTypes = append(invalidTypes, match[1])
			}
		}
	}

	if len(invalidTypes) > 0 {
		return &InvalidTaskTypeError{Type: strings.Join(invalidTypes, ", ")}
	}
	return nil
}

// containsHumanActionTask checks if output mentions any human-action task types
// Used for soft failure analysis to detect when Claude hit a manual checkpoint
func containsHumanActionTask(output string) bool {
	// Check for explicit "MANUAL CHECKPOINT" marker (used by Claude when presenting checkpoints)
	if strings.Contains(output, "MANUAL CHECKPOINT") {
		return true
	}

	// Check for any task type that requires human action
	for _, taskType := range ValidTaskTypes {
		if taskType.RequiresHumanAction() {
			// Check both the raw type string and the XML attribute format
			if strings.Contains(output, string(taskType)) {
				return true
			}
			if strings.Contains(output, `type="`+string(taskType)+`"`) {
				return true
			}
		}
	}
	return false
}
