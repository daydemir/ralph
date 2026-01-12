package workspace

import (
	"errors"
	"os"
	"path/filepath"
)

const RalphDir = ".ralph"

var ErrNoWorkspace = errors.New("no ralph workspace found (run 'ralph init' first)")
var ErrWorkspaceExists = errors.New("ralph workspace already exists (use --force to overwrite)")

// Find walks up from cwd looking for .ralph/ directory
func Find() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		ralphPath := filepath.Join(dir, RalphDir)
		if info, err := os.Stat(ralphPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoWorkspace
		}
		dir = parent
	}
}

// Path returns the .ralph directory path for a workspace
func Path(workspaceDir string) string {
	return filepath.Join(workspaceDir, RalphDir)
}

// ConfigPath returns the config.yaml path
func ConfigPath(workspaceDir string) string {
	return filepath.Join(workspaceDir, RalphDir, "config.yaml")
}

// PRDPath returns the prd.json path
func PRDPath(workspaceDir string) string {
	return filepath.Join(workspaceDir, RalphDir, "prd.json")
}
