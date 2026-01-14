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

// FailureSignal represents a detected failure in Claude's output
type FailureSignal struct {
	Type   string // "task_failed", "plan_failed", "blocked"
	Detail string // The specific name/reason from the signal
}

// TokenStats tracks token usage during execution
type TokenStats struct {
	InputTokens     int
	OutputTokens    int
	TotalTokens     int
	CacheReadTokens int
}

// OutputHandler handles parsed stream events
type OutputHandler interface {
	OnToolUse(name string)
	OnText(text string)
	OnSelectedPRD(id string)
	OnDone(result string)
	OnIterationComplete()
	OnRalphComplete()
	OnFailure(signal FailureSignal)
	OnTokenUsage(usage TokenStats)
	IsIterationComplete() bool
	IsRalphComplete() bool
	HasFailed() bool
	GetFailure() *FailureSignal
	GetTokenStats() TokenStats
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
	Usage   *UsageBlock    `json:"usage,omitempty"`
}

// ContentBlock represents a content block (text or tool_use)
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"` // for tool_use
}

// UsageBlock represents token usage data from Claude's output
type UsageBlock struct {
	InputTokens            int `json:"input_tokens"`
	OutputTokens           int `json:"output_tokens"`
	CacheCreationTokens    int `json:"cache_creation_input_tokens"`
	CacheReadTokens        int `json:"cache_read_input_tokens"`
}

// ConsoleHandler implements OutputHandler for terminal output
type ConsoleHandler struct {
	toolCount         int
	iterationComplete bool
	ralphComplete     bool
	failure           *FailureSignal
	tokenStats        TokenStats
	tokenThreshold    int
	planComplete      bool
}

func NewConsoleHandler() *ConsoleHandler {
	return &ConsoleHandler{
		tokenThreshold: 120000, // 120K safety net
	}
}

// NewConsoleHandlerWithThreshold creates a handler with custom token threshold
func NewConsoleHandlerWithThreshold(threshold int) *ConsoleHandler {
	return &ConsoleHandler{
		tokenThreshold: threshold,
	}
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

func (h *ConsoleHandler) OnFailure(signal FailureSignal) {
	h.failure = &signal
}

func (h *ConsoleHandler) OnTokenUsage(usage TokenStats) {
	h.tokenStats.InputTokens += usage.InputTokens
	h.tokenStats.OutputTokens += usage.OutputTokens
	h.tokenStats.CacheReadTokens += usage.CacheReadTokens
	h.tokenStats.TotalTokens = h.tokenStats.InputTokens + h.tokenStats.OutputTokens
}

func (h *ConsoleHandler) HasFailed() bool {
	return h.failure != nil
}

func (h *ConsoleHandler) GetFailure() *FailureSignal {
	return h.failure
}

func (h *ConsoleHandler) GetTokenStats() TokenStats {
	return h.tokenStats
}

// ShouldBailOut returns true if token usage exceeds threshold
func (h *ConsoleHandler) ShouldBailOut() bool {
	return h.tokenStats.TotalTokens >= h.tokenThreshold
}

// IsPlanComplete returns true if ###PLAN_COMPLETE### was signaled
func (h *ConsoleHandler) IsPlanComplete() bool {
	return h.planComplete
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
	planCompletePattern := regexp.MustCompile(`###PLAN_COMPLETE###`)
	taskFailedPattern := regexp.MustCompile(`###TASK_FAILED:([^#]+)###`)
	planFailedPattern := regexp.MustCompile(`###PLAN_FAILED:([^#]+)###`)
	blockedPattern := regexp.MustCompile(`###BLOCKED:([^#]+)###`)
	bailoutPattern := regexp.MustCompile(`###BAILOUT:([^#]+)###`)

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
				// Parse token usage
				if event.Message.Usage != nil {
					handler.OnTokenUsage(TokenStats{
						InputTokens:     event.Message.Usage.InputTokens,
						OutputTokens:    event.Message.Usage.OutputTokens,
						CacheReadTokens: event.Message.Usage.CacheReadTokens,
					})
				}

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
						if planCompletePattern.MatchString(content.Text) {
							// Plan complete is tracked via ConsoleHandler's planComplete field
							if ch, ok := handler.(*ConsoleHandler); ok {
								ch.planComplete = true
							}
						}
						// Check for failure signals
						if match := taskFailedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "task_failed", Detail: strings.TrimSpace(match[1])})
						}
						if match := planFailedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "plan_failed", Detail: strings.TrimSpace(match[1])})
						}
						if match := blockedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "blocked", Detail: strings.TrimSpace(match[1])})
						}
						if match := bailoutPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "bailout", Detail: strings.TrimSpace(match[1])})
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
			if planCompletePattern.MatchString(event.Result) {
				if ch, ok := handler.(*ConsoleHandler); ok {
					ch.planComplete = true
				}
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
