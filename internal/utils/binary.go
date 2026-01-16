package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveBinaryPath finds a binary, checking common locations
func ResolveBinaryPath(binaryPath string) string {
	// If it's an absolute path, use it directly
	if filepath.IsAbs(binaryPath) {
		return binaryPath
	}

	// Check if it's in PATH
	if path, err := exec.LookPath(binaryPath); err == nil {
		return path
	}

	// Handle tilde prefix
	if strings.HasPrefix(binaryPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, binaryPath[1:])
		}
	}

	// Check common locations
	home, err := os.UserHomeDir()
	if err == nil {
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
	}

	// Return original, will fail with helpful error later
	return binaryPath
}

// ClaudeNotFoundError returns a helpful error message when Claude is not found
func ClaudeNotFoundError() error {
	return fmt.Errorf(`claude not found in PATH

To fix, add to your ~/.zshrc or ~/.bashrc:
  export PATH="$HOME/.claude/local:$PATH"

Then restart your terminal, or run:
  source ~/.zshrc

Alternatively, set the full path in .ralph/config.yaml:
  claude:
    binary: /path/to/claude`)
}
