package types

import (
	"fmt"
	"time"
)

// Context represents project-level context (context.json)
type Context struct {
	Version     string    `json:"version"`
	Summary     string    `json:"summary"`     // Markdown project summary
	TechStack   []string  `json:"tech_stack"`
	Conventions []string  `json:"conventions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate ensures the context is valid
func (c *Context) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("context.version: field is required")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("context.created_at: field is required")
	}
	if c.UpdatedAt.IsZero() {
		return fmt.Errorf("context.updated_at: field is required")
	}
	return nil
}
