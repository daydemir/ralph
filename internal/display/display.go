// Package display provides unified output formatting for Ralph CLI.
// It visually separates Ralph orchestration messages from Claude Code output.
package display

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// Display handles all CLI output with visual hierarchy
type Display struct {
	theme     *Theme
	termWidth int
	noColor   bool
}

// TokenStats holds token usage info for display
type TokenStats struct {
	TotalTokens int
	Threshold   int
}

// New creates a new Display instance
func New() *Display {
	return NewWithOptions(false)
}

// NewWithOptions creates a Display with configuration
func NewWithOptions(noColor bool) *Display {
	d := &Display{
		termWidth: getTerminalWidth(),
		noColor:   noColor,
	}
	if noColor {
		d.theme = NoColorTheme()
	} else {
		d.theme = DefaultTheme()
	}
	return d
}

// getTerminalWidth returns the terminal width, defaulting to 80
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 40 {
		return 80
	}
	if width > 120 {
		return 120 // Cap at 120 for readability
	}
	return width
}

// Ralph prints a boxed message for Ralph orchestration output
func (d *Display) Ralph(lines ...string) {
	d.RalphBox("RALPH", lines...)
}

// RalphBox prints a boxed message with a custom title
func (d *Display) RalphBox(title string, lines ...string) {
	if len(lines) == 0 {
		return
	}

	width := d.termWidth - 2
	titleLen := len(title) + 4 // "─ TITLE "
	remainingWidth := width - titleLen

	// Top border: ┌─ RALPH ─────────────────────────┐
	topLine := BoxTopLeft + BoxHorizontal + " " + title + " " + strings.Repeat(BoxHorizontal, remainingWidth) + BoxTopRight
	fmt.Println(d.theme.RalphBorder(topLine))

	// Content lines: │ text                            │
	for _, line := range lines {
		paddedLine := d.padRight(line, width-2)
		fmt.Println(d.theme.RalphBorder(BoxVertical) + " " + d.theme.RalphText(paddedLine) + " " + d.theme.RalphBorder(BoxVertical))
	}

	// Bottom border: └─────────────────────────────────┘
	bottomLine := BoxBottomLeft + strings.Repeat(BoxHorizontal, width) + BoxBottomRight
	fmt.Println(d.theme.RalphBorder(bottomLine))
}

// RalphStatus prints a single-line Ralph status message (no box)
func (d *Display) RalphStatus(symbol, message string) {
	timestamp := time.Now().Format("[15:04:05]")
	fmt.Printf("%s %s %s\n",
		d.theme.RalphBorder(timestamp),
		symbol,
		d.theme.RalphText(message))
}

// Success prints a success message with green checkmark
func (d *Display) Success(message string) {
	d.RalphStatus(d.theme.Success(SymbolSuccess), message)
}

// Error prints an error message with red X
func (d *Display) Error(message string) {
	d.RalphStatus(d.theme.Error(SymbolError), message)
}

// Warning prints a warning message with yellow triangle
func (d *Display) Warning(message string) {
	d.RalphStatus(d.theme.Warning(SymbolWarning), message)
}

// Info prints an info message with cyan indicator
func (d *Display) Info(label, message string) {
	d.RalphStatus(d.theme.Info(label+":"), message)
}

// Resume prints a resume/bailout message with cyan arrow
func (d *Display) Resume(message string) {
	d.RalphStatus(d.theme.Info(SymbolResume), message)
}

// ClaudeStart prints a header when Claude execution begins
func (d *Display) ClaudeStart() {
	timestamp := time.Now().Format("[15:04:05]")
	fmt.Printf("  %s %s Sending to Claude...\n",
		d.theme.Dim(timestamp),
		d.theme.ClaudeTimestamp(GutterClaude))
}

// wrapText wraps text to specified width, returns up to maxLines
func (d *Display) wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	text = strings.TrimSpace(text)
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len()+len(word)+1 > maxWidth {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
		}
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	// Limit to 5 lines
	if len(lines) > 5 {
		lines = lines[:5]
		if len(lines[4]) > maxWidth-3 {
			lines[4] = lines[4][:maxWidth-3]
		}
		lines[4] = lines[4] + "..."
	}

	return lines
}

// Claude prints Claude Code output with left gutter indicator
func (d *Display) Claude(text string, toolCount int) {
	timestamp := time.Now().Format("[15:04:05]")
	gutter := d.theme.ClaudeTimestamp(GutterClaude)

	toolStr := ""
	if toolCount > 0 {
		toolStr = fmt.Sprintf(" %s", d.theme.ClaudeToolCount(fmt.Sprintf("[%d]", toolCount)))
	}

	lines := d.wrapText(text, d.termWidth-20)

	for i, line := range lines {
		if i == 0 {
			fmt.Printf("  %s %s%s %s\n", gutter, d.theme.Dim(timestamp), toolStr, d.theme.ClaudeText(line))
		} else {
			fmt.Printf("  %s %s%s\n", d.theme.ClaudeTimestamp(GutterDot), strings.Repeat(" ", 10), d.theme.ClaudeText(line))
		}
	}
}

// ClaudeWithTokens prints Claude Code output with token stats
func (d *Display) ClaudeWithTokens(text string, toolCount int, tokens TokenStats) {
	timestamp := time.Now().Format("[15:04:05]")
	gutter := d.theme.ClaudeTimestamp(GutterClaude)

	toolStr := ""
	if toolCount > 0 {
		toolStr = fmt.Sprintf(" %s", d.theme.ClaudeToolCount(fmt.Sprintf("[%d]", toolCount)))
	}

	// Add token display: [42K/120K]
	tokenStr := fmt.Sprintf(" %s", d.theme.Dim(fmt.Sprintf("[%dK/%dK]", tokens.TotalTokens/1000, tokens.Threshold/1000)))

	lines := d.wrapText(text, d.termWidth-30)

	for i, line := range lines {
		if i == 0 {
			fmt.Printf("  %s %s%s%s %s\n", gutter, d.theme.Dim(timestamp), toolStr, tokenStr, d.theme.ClaudeText(line))
		} else {
			fmt.Printf("  %s %s%s\n", d.theme.ClaudeTimestamp(GutterDot), strings.Repeat(" ", 20), d.theme.ClaudeText(line))
		}
	}
}

// ClaudeDone prints Claude completion message (indented)
func (d *Display) ClaudeDone(result string) {
	timestamp := time.Now().Format("[15:04:05]")
	line := fmt.Sprintf("%s%s %s %s",
		IndentClaude,
		d.theme.ClaudeTimestamp(timestamp),
		d.theme.ClaudeToolCount("[Done]"),
		d.theme.ClaudeText(result))
	fmt.Println(line)
}

// ClaudeWorkingOn prints the "WORKING ON" banner for PRD selection
func (d *Display) ClaudeWorkingOn(id string) {
	banner := fmt.Sprintf(">>> WORKING ON: %s <<<", id)
	fmt.Printf("\n%s%s\n\n", IndentClaude, d.theme.RalphLabel(banner))
}

// SectionBreak prints a horizontal separator for iteration boundaries
func (d *Display) SectionBreak() {
	width := d.termWidth
	fmt.Println(d.theme.Separator(strings.Repeat(SectionBreak, width)))
}

// Iteration prints the iteration banner with progress
func (d *Display) Iteration(current, max int, planName string, completed, total int) {
	d.SectionBreak()
	line := fmt.Sprintf("Iteration %d/%d: %s (%d/%d plans done)",
		current, max, d.theme.Info(planName), completed, total)
	fmt.Println(line)
	d.SectionBreak()
}

// LoopHeader prints the loop mode header
func (d *Display) LoopHeader() {
	fmt.Println(d.theme.Bold("=== Ralph Autonomous Loop ==="))
	fmt.Println()
}

// AllComplete prints the completion message
func (d *Display) AllComplete() {
	fmt.Printf("\n%s All plans complete!\n", d.theme.Success(SymbolSuccess))
}

// LoopComplete prints the loop completion message
func (d *Display) LoopComplete(message string, completed int) {
	fmt.Printf("\n%s %s\n", d.theme.Success(SymbolSuccess), message)
	fmt.Printf("   %d plans completed.\n", completed)
}

// LoopFailed prints the loop failure message
func (d *Display) LoopFailed(planName string, err error, completed int) {
	fmt.Printf("\n%s FAILED: %s\n", d.theme.Error(SymbolError), planName)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
	fmt.Printf("\nStopping loop. %d plans complete, 1 failed.\n", completed)
	fmt.Println("Run 'ralph status' for details.")
}

// MaxIterations prints the max iterations reached message
func (d *Display) MaxIterations(max int) {
	fmt.Printf("\nReached max iterations (%d). Run 'ralph run --loop' to continue.\n", max)
}

// Tokens prints token usage stats in a Ralph box
func (d *Display) Tokens(total, input, output int) {
	line := fmt.Sprintf("Tokens: %d (in: %d, out: %d)", total, input, output)
	d.RalphStatus(d.theme.Dim(""), line)
}

// Duration prints execution duration
func (d *Display) Duration(dur time.Duration) {
	fmt.Printf("   Duration: %s\n", dur.Round(time.Second))
}

// Theme returns the current theme for external use
func (d *Display) Theme() *Theme {
	return d.theme
}

// padRight pads a string to the specified width
func (d *Display) padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Truncate truncates text to max length with ellipsis
func Truncate(s string, max int) string {
	s = CleanText(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// CleanText removes newlines and collapses spaces
func CleanText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// AnalysisStart prints header when analysis begins
func (d *Display) AnalysisStart(observationCount int) {
	timestamp := time.Now().Format("[15:04:05]")
	fmt.Printf("\n%s %s %s\n",
		d.theme.Dim(timestamp),
		d.theme.AnalysisGutter(GutterAnalysis),
		d.theme.AnalysisText(fmt.Sprintf("Analyzing %d observations...", observationCount)))
}

// Analysis prints analysis output with distinct styling
func (d *Display) Analysis(text string) {
	lines := d.wrapText(text, d.termWidth-15)
	for i, line := range lines {
		if i == 0 {
			fmt.Printf("  %s %s\n", d.theme.AnalysisGutter(GutterAnalysis), d.theme.AnalysisText(line))
		} else {
			fmt.Printf("  %s %s\n", d.theme.AnalysisGutter(GutterDot), d.theme.AnalysisText(line))
		}
	}
}

// AnalysisComplete prints analysis completion
func (d *Display) AnalysisComplete(modified, newPlans int) {
	timestamp := time.Now().Format("[15:04:05]")
	fmt.Printf("%s %s %s\n",
		d.theme.Dim(timestamp),
		d.theme.AnalysisGutter(GutterAnalysis),
		d.theme.Success(fmt.Sprintf("Analysis complete (modified: %d, new: %d)", modified, newPlans)))
}
