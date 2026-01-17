package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VerbatimExtractor extracts conversation logs from Claude Code's log files
type VerbatimExtractor struct {
	claudeProjectPath string // ~/.claude/projects/{project}/
	planningDir       string // .planning/
	verbatimDir       string // .planning/verbatim/
}

// SessionInfo holds metadata about a Claude session
type SessionInfo struct {
	ID        string
	Path      string
	StartTime time.Time
	EndTime   time.Time
	Entries   int
}

// LogEntry represents a single entry in the JSONL log file
type LogEntry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Message   json.RawMessage `json:"message"`
	// For assistant messages, content is an array
	// For user messages, content is a string
}

// MessageContent for parsing assistant messages
type MessageContent struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// TextBlock for parsing assistant content blocks
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewVerbatimExtractor creates a new extractor for the given project
func NewVerbatimExtractor(workDir, planningDir string) (*VerbatimExtractor, error) {
	// Find Claude's project folder
	// Format: ~/.claude/projects/-Users-{user}-{project-path}/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get home directory: %w", err)
	}

	// Convert workDir to Claude's folder naming convention
	// /Users/suelio/Local/mix -> -Users-suelio-Local-mix
	claudeFolderName := strings.ReplaceAll(workDir, "/", "-")
	if strings.HasPrefix(claudeFolderName, "-") {
		claudeFolderName = claudeFolderName[1:] // Remove leading dash
	}
	claudeFolderName = "-" + claudeFolderName // Add it back properly

	claudeProjectPath := filepath.Join(homeDir, ".claude", "projects", claudeFolderName)

	// Verify path exists
	if _, err := os.Stat(claudeProjectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Claude project folder not found at %s", claudeProjectPath)
	}

	verbatimDir := filepath.Join(planningDir, "verbatim")

	return &VerbatimExtractor{
		claudeProjectPath: claudeProjectPath,
		planningDir:       planningDir,
		verbatimDir:       verbatimDir,
	}, nil
}

// GetSessions returns all session files in the Claude project folder
func (e *VerbatimExtractor) GetSessions() ([]SessionInfo, error) {
	entries, err := os.ReadDir(e.claudeProjectPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read Claude project folder: %w", err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		sessionPath := filepath.Join(e.claudeProjectPath, entry.Name())

		// Get file info for timestamps
		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			ID:      sessionID,
			Path:    sessionPath,
			EndTime: info.ModTime(),
		})
	}

	// Sort by modification time (most recent last)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].EndTime.Before(sessions[j].EndTime)
	})

	return sessions, nil
}

// GetLatestSession returns the most recently modified session
func (e *VerbatimExtractor) GetLatestSession() (*SessionInfo, error) {
	sessions, err := e.GetSessions()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	return &sessions[len(sessions)-1], nil
}

// ExtractSession extracts a specific session to a markdown file
func (e *VerbatimExtractor) ExtractSession(sessionID string) (string, error) {
	sessionPath := filepath.Join(e.claudeProjectPath, sessionID+".jsonl")
	return e.extractSessionFromPath(sessionPath, sessionID)
}

// ExtractLatest extracts the most recent session
func (e *VerbatimExtractor) ExtractLatest() (string, error) {
	session, err := e.GetLatestSession()
	if err != nil {
		return "", err
	}

	return e.extractSessionFromPath(session.Path, session.ID)
}

// extractSessionFromPath does the actual extraction work
func (e *VerbatimExtractor) extractSessionFromPath(sessionPath, sessionID string) (string, error) {
	// Ensure verbatim directory exists
	if err := os.MkdirAll(e.verbatimDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create verbatim directory: %w", err)
	}

	// Open session file
	file, err := os.Open(sessionPath)
	if err != nil {
		return "", fmt.Errorf("cannot open session file: %w", err)
	}
	defer file.Close()

	// Parse entries
	var entries []struct {
		Type      string
		Timestamp time.Time
		Content   string
	}

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max line size

	var firstTimestamp time.Time

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed entries
		}

		// Parse timestamp
		ts, _ := time.Parse(time.RFC3339, entry.Timestamp)
		if firstTimestamp.IsZero() {
			firstTimestamp = ts
		}

		// Only process user and assistant messages
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}

		// Extract content based on type
		var content string
		if entry.Type == "user" {
			// User messages: message.content is a string
			var msg struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(entry.Message, &msg); err == nil {
				content = msg.Content
			}
		} else if entry.Type == "assistant" {
			// Assistant messages: message.content is an array of content blocks
			var msg struct {
				Content []TextBlock `json:"content"`
			}
			if err := json.Unmarshal(entry.Message, &msg); err == nil {
				var textParts []string
				for _, block := range msg.Content {
					if block.Type == "text" {
						textParts = append(textParts, block.Text)
					}
				}
				content = strings.Join(textParts, "\n\n")
			}
		}

		if content != "" {
			entries = append(entries, struct {
				Type      string
				Timestamp time.Time
				Content   string
			}{
				Type:      entry.Type,
				Timestamp: ts,
				Content:   content,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading session file: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no messages found in session")
	}

	// Generate output filename
	dateStr := firstTimestamp.Format("2006-01-02")
	timeStr := firstTimestamp.Format("15-04-05")
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	outFileName := fmt.Sprintf("%s-%s-%s.md", dateStr, timeStr, shortID)
	outPath := filepath.Join(e.verbatimDir, outFileName)

	// Build markdown content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Session %s\n\n", firstTimestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Session ID: `%s`\n\n", sessionID))
	sb.WriteString("---\n\n")

	for _, entry := range entries {
		role := "USER"
		if entry.Type == "assistant" {
			role = "CLAUDE"
		}
		timeStr := entry.Timestamp.Format("15:04:05")
		sb.WriteString(fmt.Sprintf("## %s (%s)\n\n", role, timeStr))
		sb.WriteString(entry.Content)
		sb.WriteString("\n\n---\n\n")
	}

	// Write output file
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("cannot write output file: %w", err)
	}

	return outPath, nil
}

// GetSessionsForDateRange returns sessions within a date range
func (e *VerbatimExtractor) GetSessionsForDateRange(from, to time.Time) ([]SessionInfo, error) {
	allSessions, err := e.GetSessions()
	if err != nil {
		return nil, err
	}

	var filtered []SessionInfo
	for _, session := range allSessions {
		if (session.EndTime.After(from) || session.EndTime.Equal(from)) &&
			(session.EndTime.Before(to) || session.EndTime.Equal(to)) {
			filtered = append(filtered, session)
		}
	}

	return filtered, nil
}

// ClaudeProjectPath returns the Claude project folder path (for display)
func (e *VerbatimExtractor) ClaudeProjectPath() string {
	return e.claudeProjectPath
}
