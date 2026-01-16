package types

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation error with structured information
type ValidationError struct {
	Field    string      // JSON field path like "tasks[0].type"
	Expected string      // What was expected: "one of: auto, manual"
	Actual   interface{} // What was found
	Message  string      // Human-readable description
}

// ValidationErrors is a collection of validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

// Add appends a new validation error to the collection
func (v *ValidationErrors) Add(field, expected string, actual interface{}, msg string) {
	v.Errors = append(v.Errors, ValidationError{
		Field:    field,
		Expected: expected,
		Actual:   actual,
		Message:  msg,
	})
}

// HasErrors returns true if there are any validation errors
func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// Error implements the error interface
// Returns a simple error message for standard error handling
func (v *ValidationErrors) Error() string {
	if !v.HasErrors() {
		return "no validation errors"
	}

	if len(v.Errors) == 1 {
		e := v.Errors[0]
		return fmt.Sprintf("validation error in field %s: %s", e.Field, e.Message)
	}

	return fmt.Sprintf("validation failed with %d errors", len(v.Errors))
}

// ToPrompt formats validation errors for Claude consumption
// This produces a clear, actionable message that Claude can use to fix the JSON
func (v *ValidationErrors) ToPrompt() string {
	if !v.HasErrors() {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Validation failed with %d error(s):\n\n", len(v.Errors)))

	for i, err := range v.Errors {
		sb.WriteString(fmt.Sprintf("%d. Field: %s\n", i+1, err.Field))
		sb.WriteString(fmt.Sprintf("   Expected: %s\n", err.Expected))
		sb.WriteString(fmt.Sprintf("   Found: %v\n", formatActual(err.Actual)))
		sb.WriteString(fmt.Sprintf("   Fix: %s\n", err.Message))

		if i < len(v.Errors)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatActual formats the actual value for display
func formatActual(actual interface{}) string {
	if actual == nil {
		return "null"
	}

	switch v := actual.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case []string:
		if len(v) == 0 {
			return "[]"
		}
		quoted := make([]string, len(v))
		for i, s := range v {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	default:
		return fmt.Sprintf("%v", actual)
	}
}
