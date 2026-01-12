package llm

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Claude implements the Backend interface for Claude Code CLI
type Claude struct {
	BinaryPath string
}

// NewClaude creates a new Claude backend
func NewClaude(binaryPath string) *Claude {
	if binaryPath == "" {
		binaryPath = "claude"
	}
	// Try to resolve the binary path
	resolved := resolveBinaryPath(binaryPath)
	return &Claude{BinaryPath: resolved}
}

// resolveBinaryPath finds the claude binary, checking common locations
func resolveBinaryPath(binaryPath string) string {
	// If it's an absolute path, use it directly
	if filepath.IsAbs(binaryPath) {
		return binaryPath
	}

	// Check if it's in PATH
	if path, err := exec.LookPath(binaryPath); err == nil {
		return path
	}

	// Check common locations
	home, _ := os.UserHomeDir()
	commonPaths := []string{
		filepath.Join(home, ".claude", "local", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Return original, will fail with helpful error later
	return binaryPath
}

// claudeNotFoundError returns a helpful error message
func claudeNotFoundError() error {
	return fmt.Errorf(`claude not found in PATH

To fix, add to your ~/.zshrc or ~/.bashrc:
  export PATH="$HOME/.claude/local:$PATH"

Then restart your terminal, or run:
  source ~/.zshrc

Alternatively, set the full path in .ralph/config.yaml:
  claude:
    binary: /path/to/claude`)
}

func (c *Claude) Name() string {
	return "claude"
}

// Execute runs Claude Code with the given options and returns streaming output
func (c *Claude) Execute(ctx context.Context, opts ExecuteOptions) (io.ReadCloser, error) {
	args := c.buildArgs(opts, false)

	cmd := exec.CommandContext(ctx, c.BinaryPath, args...)
	cmd.Dir = opts.WorkDir
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, claudeNotFoundError()
		}
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Return a wrapper that waits for the command when closed
	return &cmdReader{
		ReadCloser: stdout,
		cmd:        cmd,
	}, nil
}

// ExecuteInteractive runs Claude Code in interactive mode
func (c *Claude) ExecuteInteractive(ctx context.Context, opts ExecuteOptions) error {
	args := c.buildArgs(opts, true)

	cmd := exec.CommandContext(ctx, c.BinaryPath, args...)
	cmd.Dir = opts.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (c *Claude) buildArgs(opts ExecuteOptions, interactive bool) []string {
	var args []string

	// Model
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// Prompt (only for non-interactive)
	if !interactive && opts.Prompt != "" {
		args = append(args, "-p", opts.Prompt)
	}

	// Allowed tools
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}

	// Output format (only for non-interactive)
	if !interactive {
		args = append(args, "--output-format", "stream-json", "--verbose")
	}

	// Context files
	args = append(args, opts.ContextFiles...)

	return args
}

// cmdReader wraps an io.ReadCloser and waits for the command on close
type cmdReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (r *cmdReader) Close() error {
	r.ReadCloser.Close()
	return r.cmd.Wait()
}
