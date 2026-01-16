package llm

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/daydemir/ralph/internal/utils"
)

// ExecuteOptions contains options for Claude execution
type ExecuteOptions struct {
	Prompt       string
	ContextFiles []string
	Model        string
	AllowedTools []string
	WorkDir      string
}

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
	resolved := utils.ResolveBinaryPath(binaryPath)
	return &Claude{BinaryPath: resolved}
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
			return nil, utils.ClaudeNotFoundError()
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

	// Skip permissions for autonomous execution
	args = append(args, "--dangerously-skip-permissions")

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
	closeErr := r.ReadCloser.Close()
	waitErr := r.cmd.Wait()
	if waitErr != nil {
		return waitErr
	}
	return closeErr
}
