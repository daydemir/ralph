package executor

import (
	"regexp"
	"strings"

	"github.com/daydemir/ralph/internal/types"
)

// TaskType is an alias to the canonical types.TaskType
type TaskType = types.TaskType

// Task type constants - re-exported from types package for convenience
const (
	TaskTypeAuto   = types.TaskTypeAuto
	TaskTypeManual = types.TaskTypeManual
)

// ValidTaskTypes is the exhaustive list of allowed task types
var ValidTaskTypes = types.AllTaskTypes()

// RequiresHumanAction returns true if this task type requires human intervention
func RequiresHumanAction(t TaskType) bool {
	return t == TaskTypeManual
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
	suggestion := ""
	// Suggest migration for common invalid types
	if strings.Contains(e.Type, "checkpoint:") || strings.Contains(e.Type, "human") {
		suggestion = " Use type=\"manual\" instead for human verification/decision tasks."
	}
	return "invalid task type '" + e.Type + "'. Valid: auto, manual." + suggestion
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
// Used for soft failure analysis to detect when Claude hit a manual task
func containsHumanActionTask(output string) bool {
	// Check for explicit "MANUAL" markers
	if strings.Contains(output, "MANUAL CHECKPOINT") || strings.Contains(output, "MANUAL TASK") {
		return true
	}

	// Check for manual task type in output
	if strings.Contains(output, string(TaskTypeManual)) {
		return true
	}
	if strings.Contains(output, `type="manual"`) {
		return true
	}
	return false
}
