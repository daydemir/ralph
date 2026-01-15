package llm

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/daydemir/ralph/internal/display"
)

// FailureSignal represents a detected failure in Claude's output
type FailureSignal struct {
	Type   string // "task_failed", "plan_failed", "blocked", "bailout", "build_failed", "test_failed"
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
	display           *display.Display
	toolCount         int
	iterationComplete bool
	ralphComplete     bool
	failure           *FailureSignal
	tokenStats        TokenStats
	tokenThreshold    int
	planComplete      bool
	bailoutSignal     *FailureSignal // Separate tracking for BAILOUT (soft failure)
	lastDoneText      string         // Track last done message to prevent duplicates
}

func NewConsoleHandler() *ConsoleHandler {
	return &ConsoleHandler{
		display:        display.New(),
		tokenThreshold: 120000, // 120K safety net
	}
}

// NewConsoleHandlerWithThreshold creates a handler with custom token threshold
func NewConsoleHandlerWithThreshold(threshold int) *ConsoleHandler {
	return &ConsoleHandler{
		display:        display.New(),
		tokenThreshold: threshold,
	}
}

// NewConsoleHandlerWithDisplay creates a handler with a shared display instance
func NewConsoleHandlerWithDisplay(d *display.Display) *ConsoleHandler {
	return &ConsoleHandler{
		display:        d,
		tokenThreshold: 120000,
	}
}

func (h *ConsoleHandler) OnToolUse(name string) {
	h.toolCount++
}

func (h *ConsoleHandler) OnText(text string) {
	truncated := display.Truncate(text, 400)
	h.display.Claude(truncated, h.toolCount)
	h.toolCount = 0
}

func (h *ConsoleHandler) OnSelectedPRD(id string) {
	h.display.ClaudeWorkingOn(id)
}

func (h *ConsoleHandler) OnDone(result string) {
	truncated := display.Truncate(result, 200)

	// Skip if identical to last done message
	if truncated == h.lastDoneText {
		return
	}
	h.lastDoneText = truncated

	h.display.ClaudeDone(truncated)
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
	// BAILOUT is a soft signal for context preservation, not a hard failure
	if signal.Type == "bailout" {
		h.bailoutSignal = &signal
	} else {
		h.failure = &signal
	}
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

// IsBailout returns true if ###BAILOUT### was signaled (soft failure for context preservation)
func (h *ConsoleHandler) IsBailout() bool {
	return h.bailoutSignal != nil
}

// GetBailout returns the bailout signal details
func (h *ConsoleHandler) GetBailout() *FailureSignal {
	return h.bailoutSignal
}

// ParseStream reads the Claude stream-json output and calls the handler
// onTerminate is called when a termination signal (bailout, hard failure) is detected
// to allow the caller to cancel the context and kill the Claude process
func ParseStream(reader io.Reader, handler OutputHandler, onTerminate func()) error {
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
	buildFailedPattern := regexp.MustCompile(`###BUILD_FAILED:([^#]+)###`)
	testFailedPattern := regexp.MustCompile(`###TEST_FAILED:([^#:]+):?([^#]*)###`)

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
						// Check for failure/termination signals
						// When detected, notify handler and terminate the Claude process immediately
						if match := taskFailedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "task_failed", Detail: strings.TrimSpace(match[1])})
							if onTerminate != nil {
								onTerminate()
							}
							return nil
						}
						if match := planFailedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "plan_failed", Detail: strings.TrimSpace(match[1])})
							if onTerminate != nil {
								onTerminate()
							}
							return nil
						}
						if match := blockedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "blocked", Detail: strings.TrimSpace(match[1])})
							if onTerminate != nil {
								onTerminate()
							}
							return nil
						}
						if match := bailoutPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "bailout", Detail: strings.TrimSpace(match[1])})
							if onTerminate != nil {
								onTerminate()
							}
							return nil
						}
						if match := buildFailedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							handler.OnFailure(FailureSignal{Type: "build_failed", Detail: strings.TrimSpace(match[1])})
							if onTerminate != nil {
								onTerminate()
							}
							return nil
						}
						if match := testFailedPattern.FindStringSubmatch(content.Text); len(match) > 1 {
							detail := strings.TrimSpace(match[1])
							if len(match) > 2 && match[2] != "" {
								detail = detail + ":" + strings.TrimSpace(match[2])
							}
							handler.OnFailure(FailureSignal{Type: "test_failed", Detail: detail})
							if onTerminate != nil {
								onTerminate()
							}
							return nil
						}
						// Output text
						handler.OnText(cleanText(content.Text))
					}
				}
			}
		case "result":
			// Check for failure/termination signals in result events too
			// These can appear when Claude outputs them in its final message
			if match := taskFailedPattern.FindStringSubmatch(event.Result); len(match) > 1 {
				handler.OnFailure(FailureSignal{Type: "task_failed", Detail: strings.TrimSpace(match[1])})
				if onTerminate != nil {
					onTerminate()
				}
				return nil
			}
			if match := planFailedPattern.FindStringSubmatch(event.Result); len(match) > 1 {
				handler.OnFailure(FailureSignal{Type: "plan_failed", Detail: strings.TrimSpace(match[1])})
				if onTerminate != nil {
					onTerminate()
				}
				return nil
			}
			if match := blockedPattern.FindStringSubmatch(event.Result); len(match) > 1 {
				handler.OnFailure(FailureSignal{Type: "blocked", Detail: strings.TrimSpace(match[1])})
				if onTerminate != nil {
					onTerminate()
				}
				return nil
			}
			if match := bailoutPattern.FindStringSubmatch(event.Result); len(match) > 1 {
				handler.OnFailure(FailureSignal{Type: "bailout", Detail: strings.TrimSpace(match[1])})
				if onTerminate != nil {
					onTerminate()
				}
				return nil
			}
			if match := buildFailedPattern.FindStringSubmatch(event.Result); len(match) > 1 {
				handler.OnFailure(FailureSignal{Type: "build_failed", Detail: strings.TrimSpace(match[1])})
				if onTerminate != nil {
					onTerminate()
				}
				return nil
			}
			if match := testFailedPattern.FindStringSubmatch(event.Result); len(match) > 1 {
				detail := strings.TrimSpace(match[1])
				if len(match) > 2 && match[2] != "" {
					detail = detail + ":" + strings.TrimSpace(match[2])
				}
				handler.OnFailure(FailureSignal{Type: "test_failed", Detail: detail})
				if onTerminate != nil {
					onTerminate()
				}
				return nil
			}
			// Check for completion signals
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

// cleanText wraps display.CleanText for internal use
func cleanText(s string) string {
	return display.CleanText(s)
}
