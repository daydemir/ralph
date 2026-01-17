package executor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/daydemir/ralph/internal/llm"
	"github.com/daydemir/ralph/internal/types"
)

// BuildSystem represents a detected build system
type BuildSystem struct {
	Name         string   // e.g., "npm", "go", "make", "cargo"
	BuildCmd     string   // Command to build
	TestCmd      string   // Command to test
	DetectedAt   string   // Path where detected
	FromClaudeMD bool     // Whether commands came from CLAUDE.md
}

// BuildVerificationResult holds the result of build verification
type BuildVerificationResult struct {
	Success      bool
	BuildPassed  bool
	TestsPassed  bool
	BuildOutput  string
	TestOutput   string
	Error        error
	BuildSystem  *BuildSystem
	AutoFixTried bool
}

// DetectBuildSystems scans the workspace for build systems
func (e *Executor) DetectBuildSystems() []BuildSystem {
	var systems []BuildSystem

	// First check CLAUDE.md for explicit build commands
	claudeMD := e.findClaudeMD()
	if claudeMD != nil {
		systems = append(systems, *claudeMD)
	}

	// Check for common build systems
	checks := []struct {
		file      string
		name      string
		buildCmd  string
		testCmd   string
	}{
		{"package.json", "npm", "npm run build", "npm test"},
		{"go.mod", "go", "go build ./...", "go test ./..."},
		{"Cargo.toml", "cargo", "cargo build", "cargo test"},
		{"Makefile", "make", "make", "make test"},
		{"build.gradle", "gradle", "./gradlew build", "./gradlew test"},
		{"pom.xml", "maven", "mvn compile", "mvn test"},
		{"*.xcodeproj", "xcode", "xcodebuild", "xcodebuild test"},
	}

	for _, check := range checks {
		// Handle glob patterns
		var matches []string
		if strings.Contains(check.file, "*") {
			matches, _ = filepath.Glob(filepath.Join(e.config.WorkDir, check.file))
		} else {
			path := filepath.Join(e.config.WorkDir, check.file)
			if _, err := os.Stat(path); err == nil {
				matches = []string{path}
			}
		}

		if len(matches) > 0 {
			// Don't duplicate if we already have from CLAUDE.md
			alreadyHave := false
			for _, s := range systems {
				if s.Name == check.name {
					alreadyHave = true
					break
				}
			}
			if !alreadyHave {
				systems = append(systems, BuildSystem{
					Name:       check.name,
					BuildCmd:   check.buildCmd,
					TestCmd:    check.testCmd,
					DetectedAt: matches[0],
				})
			}
		}
	}

	return systems
}

// findClaudeMD looks for CLAUDE.md and extracts build commands
func (e *Executor) findClaudeMD() *BuildSystem {
	// Check common locations for CLAUDE.md
	locations := []string{
		filepath.Join(e.config.WorkDir, "CLAUDE.md"),
		filepath.Join(e.config.WorkDir, ".claude", "CLAUDE.md"),
		filepath.Join(e.config.PlanningDir, "CLAUDE.md"),
	}

	for _, path := range locations {
		if content, err := os.ReadFile(path); err == nil {
			buildCmd, testCmd := extractCommandsFromClaudeMD(string(content))
			if buildCmd != "" || testCmd != "" {
				return &BuildSystem{
					Name:         "claude.md",
					BuildCmd:     buildCmd,
					TestCmd:      testCmd,
					DetectedAt:   path,
					FromClaudeMD: true,
				}
			}
		}
	}

	return nil
}

// extractCommandsFromClaudeMD parses CLAUDE.md for build/test commands
func extractCommandsFromClaudeMD(content string) (buildCmd, testCmd string) {
	lines := strings.Split(content, "\n")
	inBuildSection := false
	inTestSection := false

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Detect sections
		if strings.Contains(lower, "build") && (strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##")) {
			inBuildSection = true
			inTestSection = false
			continue
		}
		if strings.Contains(lower, "test") && (strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##")) {
			inTestSection = true
			inBuildSection = false
			continue
		}
		if strings.HasPrefix(line, "#") {
			inBuildSection = false
			inTestSection = false
			continue
		}

		// Extract commands from code blocks
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			continue
		}

		// Look for command patterns
		if inBuildSection && buildCmd == "" {
			if isCommand(trimmed) {
				buildCmd = trimmed
			}
		}
		if inTestSection && testCmd == "" {
			if isCommand(trimmed) {
				testCmd = trimmed
			}
		}
	}

	return buildCmd, testCmd
}

// isCommand checks if a line looks like a shell command
func isCommand(line string) bool {
	if line == "" {
		return false
	}
	// Common command prefixes
	prefixes := []string{"npm", "yarn", "pnpm", "go", "make", "cargo", "mvn", "gradle", "xcodebuild", "swift", "python", "pytest", "ruby", "bundle"}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p+" ") || line == p {
			return true
		}
	}
	// Or starts with ./ or $
	if strings.HasPrefix(line, "./") || strings.HasPrefix(line, "$") {
		return true
	}
	return false
}

// VerifyProjectBuilds runs build verification after plan completion
func (e *Executor) VerifyProjectBuilds(ctx context.Context) *BuildVerificationResult {
	result := &BuildVerificationResult{
		Success:     true,
		BuildPassed: true,
		TestsPassed: true,
	}

	// Detect build systems
	systems := e.DetectBuildSystems()
	if len(systems) == 0 {
		e.display.Info("Build", "No build system detected, skipping verification")
		return result
	}

	// Use the first system (prefer CLAUDE.md if found)
	system := systems[0]
	result.BuildSystem = &system

	e.display.Info("Build", fmt.Sprintf("Detected %s build system at %s", system.Name, system.DetectedAt))

	// Run build command if present
	if system.BuildCmd != "" {
		e.display.Info("Build", fmt.Sprintf("Running: %s", system.BuildCmd))

		cmd := exec.CommandContext(ctx, "bash", "-c", system.BuildCmd)
		cmd.Dir = e.config.WorkDir
		output, err := cmd.CombinedOutput()
		result.BuildOutput = string(output)

		if err != nil {
			result.BuildPassed = false
			result.Success = false
			result.Error = fmt.Errorf("build failed: %w", err)
			e.display.Error(fmt.Sprintf("Build failed: %v", err))
			return result
		}
		e.display.Success("Build passed")
	}

	// Run test command if present
	if system.TestCmd != "" {
		e.display.Info("Build", fmt.Sprintf("Running: %s", system.TestCmd))

		cmd := exec.CommandContext(ctx, "bash", "-c", system.TestCmd)
		cmd.Dir = e.config.WorkDir
		output, err := cmd.CombinedOutput()
		result.TestOutput = string(output)

		if err != nil {
			result.TestsPassed = false
			result.Success = false
			result.Error = fmt.Errorf("tests failed: %w", err)
			e.display.Error(fmt.Sprintf("Tests failed: %v", err))
			return result
		}
		e.display.Success("Tests passed")
	}

	return result
}

// TryAutoFix attempts to fix build/test failures using Claude
func (e *Executor) TryAutoFix(ctx context.Context, verifyResult *BuildVerificationResult, plan *types.Plan) *BuildVerificationResult {
	if verifyResult.Success {
		return verifyResult
	}

	e.display.Info("AutoFix", "Attempting to fix build/test failures...")
	verifyResult.AutoFixTried = true

	// Build a prompt for the auto-fix agent
	var errorOutput string
	if !verifyResult.BuildPassed {
		errorOutput = verifyResult.BuildOutput
	} else {
		errorOutput = verifyResult.TestOutput
	}

	// Truncate if too long
	if len(errorOutput) > 5000 {
		errorOutput = errorOutput[len(errorOutput)-5000:]
	}

	prompt := fmt.Sprintf(`You are an auto-fix agent. The build or tests failed after plan execution.

## Error Output
%s

## Build System
%s

## Task
1. Analyze the error output
2. Identify the root cause
3. Apply minimal fixes to resolve the issue
4. Do NOT refactor or make unnecessary changes
5. Run the failing command again to verify the fix

If you cannot fix the issue, explain why and what manual intervention is needed.

Begin fixing now.`, errorOutput, verifyResult.BuildSystem.Name)

	// Execute auto-fix with Claude
	opts := llm.ExecuteOptions{
		Prompt: prompt,
		ContextFiles: []string{
			filepath.Join(e.config.PlanningDir, "project.json"),
		},
		Model: e.config.Model,
		AllowedTools: []string{
			"Read", "Write", "Edit", "Bash", "Glob", "Grep",
		},
		WorkDir: e.config.WorkDir,
	}

	reader, err := e.claude.Execute(ctx, opts)
	if err != nil {
		e.display.Warning(fmt.Sprintf("AutoFix failed to start: %v", err))
		return verifyResult
	}
	defer reader.Close()

	// Parse output (simplified - just let it run)
	handler := llm.NewConsoleHandlerWithDisplay(e.display)
	if err := llm.ParseStream(reader, handler, nil); err != nil {
		e.display.Warning(fmt.Sprintf("AutoFix execution error: %v", err))
		return verifyResult
	}

	// Re-verify after auto-fix
	e.display.Info("AutoFix", "Re-verifying build after fix attempt...")
	newResult := e.VerifyProjectBuilds(ctx)
	newResult.AutoFixTried = true

	if newResult.Success {
		e.display.Success("AutoFix resolved the issue!")
	} else {
		e.display.Warning("AutoFix could not resolve the issue")
	}

	return newResult
}

// getBaselineTestFailures runs tests before execution to capture pre-existing failures
func (e *Executor) GetBaselineTestFailures(ctx context.Context) (map[string]bool, error) {
	failures := make(map[string]bool)

	systems := e.DetectBuildSystems()
	if len(systems) == 0 {
		return failures, nil
	}

	system := systems[0]
	if system.TestCmd == "" {
		return failures, nil
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", system.TestCmd)
	cmd.Dir = e.config.WorkDir
	output, _ := cmd.CombinedOutput()

	// Parse test output for failing test names (simplified)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		// Look for common failure patterns
		if strings.Contains(line, "FAIL") || strings.Contains(line, "FAILED") {
			failures[line] = true
		}
	}

	return failures, nil
}
