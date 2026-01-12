package llm

import (
	"context"
	"io"
)

// Backend represents an LLM execution backend
type Backend interface {
	// Name returns the backend name (e.g., "claude", "kilocode")
	Name() string

	// Execute runs the LLM with the given prompt and context files
	// Returns a reader for streaming output
	Execute(ctx context.Context, opts ExecuteOptions) (io.ReadCloser, error)

	// ExecuteInteractive runs the LLM in interactive mode (for plan command)
	ExecuteInteractive(ctx context.Context, opts ExecuteOptions) error
}

// ExecuteOptions contains options for LLM execution
type ExecuteOptions struct {
	Prompt       string
	ContextFiles []string
	Model        string
	AllowedTools []string
	WorkDir      string
}

// OutputHandler handles parsed events from the LLM output stream
type OutputHandler interface {
	OnToolUse(name string)
	OnText(text string)
	OnSelectedPRD(id string)
	OnDone(result string)
	OnIterationComplete()
	OnRalphComplete()
}
