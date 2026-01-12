package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
)

// OutputHandler handles parsed stream events
type OutputHandler interface {
	OnToolUse(name string)
	OnText(text string)
	OnSelectedPRD(id string)
	OnDone(result string)
	OnIterationComplete()
	OnRalphComplete()
	IsIterationComplete() bool
	IsRalphComplete() bool
}

// StreamEvent represents a single event from Claude's stream-json output
type StreamEvent struct {
	Type    string          `json:"type"`
	Message *MessageContent `json:"message,omitempty"`
	Result  string          `json:"result,omitempty"`
}

// MessageContent represents the message field in stream events
type MessageContent struct {
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentBlock represents a content block (text or tool_use)
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"` // for tool_use
}

// ConsoleHandler implements OutputHandler for terminal output
type ConsoleHandler struct {
	toolCount         int
	iterationComplete bool
	ralphComplete     bool
}

func NewConsoleHandler() *ConsoleHandler {
	return &ConsoleHandler{}
}

func (h *ConsoleHandler) OnToolUse(name string) {
	h.toolCount++
}

func (h *ConsoleHandler) OnText(text string) {
	timestamp := time.Now().Format("[15:04:05]")
	truncated := truncateText(text, 400)

	if h.toolCount > 0 {
		fmt.Printf("%s [Tools: %d] %s\n", timestamp, h.toolCount, truncated)
		h.toolCount = 0
	} else {
		fmt.Printf("%s %s\n", timestamp, truncated)
	}
}

func (h *ConsoleHandler) OnSelectedPRD(id string) {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Printf("\n%s\n\n", cyan(fmt.Sprintf(">>> WORKING ON: %s <<<", id)))
}

func (h *ConsoleHandler) OnDone(result string) {
	timestamp := time.Now().Format("[15:04:05]")
	truncated := truncateText(result, 200)
	fmt.Printf("%s [Done] %s\n", timestamp, truncated)
}

func (h *ConsoleHandler) OnIterationComplete() {
	h.iterationComplete = true
}

func (h *ConsoleHandler) OnRalphComplete() {
	h.ralphComplete = true
}

func (h *ConsoleHandler) IsIterationComplete() bool {
	return h.iterationComplete
}

func (h *ConsoleHandler) IsRalphComplete() bool {
	return h.ralphComplete
}

// ParseStream reads the Claude stream-json output and calls the handler
func ParseStream(reader io.Reader, handler OutputHandler) error {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	prdPattern := regexp.MustCompile(`SELECTED_PRD:\s*([a-zA-Z0-9_-]+)`)
	iterationPattern := regexp.MustCompile(`###ITERATION_COMPLETE###`)
	completePattern := regexp.MustCompile(`###RALPH_COMPLETE###`)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip malformed JSON lines
			continue
		}

		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, content := range event.Message.Content {
					switch content.Type {
					case "tool_use":
						handler.OnToolUse(content.Name)
					case "text":
						// Check for SELECTED_PRD pattern
						if match := prdPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnSelectedPRD(match[1])
						}
						// Check for completion signals
						if iterationPattern.MatchString(content.Text) {
							handler.OnIterationComplete()
						}
						if completePattern.MatchString(content.Text) {
							handler.OnRalphComplete()
						}
						// Output text
						handler.OnText(cleanText(content.Text))
					}
				}
			}
		case "result":
			if iterationPattern.MatchString(event.Result) {
				handler.OnIterationComplete()
			}
			if completePattern.MatchString(event.Result) {
				handler.OnRalphComplete()
			}
			handler.OnDone(cleanText(event.Result))
		}
	}

	return scanner.Err()
}

func truncateText(s string, max int) string {
	s = cleanText(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func cleanText(s string) string {
	// Replace newlines with spaces for single-line output
	s = strings.ReplaceAll(s, "\n", " ")
	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}
