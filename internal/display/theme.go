package display

import "github.com/fatih/color"

// Box drawing characters
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	SectionBreak   = "━"
)

// Status symbols
const (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "⚠"
	SymbolResume  = "↻"
	SymbolPending = "○"
	SymbolPartial = "◐"
)

// IndentClaude is the indentation for Claude output
const IndentClaude = "  "

// Theme holds all color functions for consistent styling
type Theme struct {
	// Ralph orchestration (prominent)
	RalphBorder  func(a ...interface{}) string
	RalphLabel   func(a ...interface{}) string
	RalphText    func(a ...interface{}) string

	// Claude output (subdued)
	ClaudeTimestamp func(a ...interface{}) string
	ClaudeText      func(a ...interface{}) string
	ClaudeToolCount func(a ...interface{}) string

	// Status indicators
	Success func(a ...interface{}) string
	Error   func(a ...interface{}) string
	Warning func(a ...interface{}) string
	Info    func(a ...interface{}) string

	// Structural elements
	Bold      func(a ...interface{}) string
	Dim       func(a ...interface{}) string
	Separator func(a ...interface{}) string
}

// DefaultTheme creates the default color theme
func DefaultTheme() *Theme {
	return &Theme{
		// Ralph orchestration - bright cyan for visibility
		RalphBorder:  color.New(color.FgCyan).SprintFunc(),
		RalphLabel:   color.New(color.FgCyan, color.Bold).SprintFunc(),
		RalphText:    color.New(color.FgWhite).SprintFunc(),

		// Claude output - dimmer/gray to distinguish from Ralph
		ClaudeTimestamp: color.New(color.FgHiBlack).SprintFunc(),
		ClaudeText:      color.New(color.FgWhite).SprintFunc(),
		ClaudeToolCount: color.New(color.FgHiBlack).SprintFunc(),

		// Status indicators
		Success: color.New(color.FgGreen).SprintFunc(),
		Error:   color.New(color.FgRed).SprintFunc(),
		Warning: color.New(color.FgYellow).SprintFunc(),
		Info:    color.New(color.FgCyan).SprintFunc(),

		// Structural
		Bold:      color.New(color.Bold).SprintFunc(),
		Dim:       color.New(color.FgHiBlack).SprintFunc(),
		Separator: color.New(color.FgCyan).SprintFunc(),
	}
}

// NoColorTheme creates a theme without colors (for --no-color flag or non-TTY)
func NoColorTheme() *Theme {
	identity := func(a ...interface{}) string {
		if len(a) == 0 {
			return ""
		}
		return a[0].(string)
	}
	return &Theme{
		RalphBorder:     identity,
		RalphLabel:      identity,
		RalphText:       identity,
		ClaudeTimestamp: identity,
		ClaudeText:      identity,
		ClaudeToolCount: identity,
		Success:         identity,
		Error:           identity,
		Warning:         identity,
		Info:            identity,
		Bold:            identity,
		Dim:             identity,
		Separator:       identity,
	}
}
