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

// KiloCode implements the Backend interface for Vibe CLI (Mistral)
type KiloCode struct {
	BinaryPath string
	APIKey     string
}

// NewKiloCode creates a new KiloCode backend
func NewKiloCode(binaryPath string, apiKey string) *KiloCode {
	if binaryPath == "" {
		binaryPath = "vibe"
	}
	// Try to resolve the binary path
	resolved := resolveBinaryPathVibe(binaryPath)
	return &KiloCode{
		BinaryPath: resolved,
		APIKey:     apiKey,
	}
}

// resolveBinaryPathVibe finds the vibe binary, checking common locations
func resolveBinaryPathVibe(binaryPath string) string {
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
		filepath.Join(home, ".vibe", "local", "vibe"),
		"/usr/local/bin/vibe",
		"/opt/homebrew/bin/vibe",
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Return original, will fail with helpful error later
	return binaryPath
}

// vibeNotFoundError returns a helpful error message
func vibeNotFoundError() error {
	return fmt.Errorf(`vibe not found in PATH

To fix, add to your ~/.zshrc or ~/.bashrc:
  export PATH="$HOME/.vibe/local:$PATH"

Then restart your terminal, or run:
  source ~/.zshrc

Alternatively, set the full path in .ralph/config.yaml:
  kilocode:
    binary: /path/to/vibe`)
}

func (k *KiloCode) Name() string {
	return "kilocode"
}

// Execute runs Vibe CLI with the given options and returns streaming output
func (k *KiloCode) Execute(ctx context.Context, opts ExecuteOptions) (io.ReadCloser, error) {
	args := k.buildArgs(opts, false)

	cmd := exec.CommandContext(ctx, k.BinaryPath, args...)
	cmd.Dir = opts.WorkDir
	cmd.Stderr = os.Stderr

	// Set MISTRAL_API_KEY environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("MISTRAL_API_KEY=%s", k.APIKey))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, vibeNotFoundError()
		}
		return nil, fmt.Errorf("failed to start vibe: %w", err)
	}

	// Return a wrapper that waits for the command when closed
	return &cmdReader{
		ReadCloser: stdout,
		cmd:        cmd,
	}, nil
}

// ExecuteInteractive runs Vibe CLI in interactive mode
func (k *KiloCode) ExecuteInteractive(ctx context.Context, opts ExecuteOptions) error {
	args := k.buildArgs(opts, true)

	cmd := exec.CommandContext(ctx, k.BinaryPath, args...)
	cmd.Dir = opts.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set MISTRAL_API_KEY environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("MISTRAL_API_KEY=%s", k.APIKey))

	return cmd.Run()
}

func (k *KiloCode) buildArgs(opts ExecuteOptions, interactive bool) []string {
	var args []string

	// Model
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// Prompt (only for non-interactive)
	if !interactive && opts.Prompt != "" {
		args = append(args, "--prompt", opts.Prompt)
	}

	// Allowed tools
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--tools", strings.Join(opts.AllowedTools, ","))
	}

	// Context files
	args = append(args, opts.ContextFiles...)

	return args
}
