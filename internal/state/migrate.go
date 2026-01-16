package state

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/daydemir/ralph/internal/types"
)

// MigrateToJSON converts existing markdown state files to JSON format
// This is idempotent - if JSON files already exist, they are skipped
func MigrateToJSON(planningDir string) error {
	// 1. Migrate roadmap if needed
	if err := migrateRoadmap(planningDir); err != nil {
		return fmt.Errorf("roadmap migration failed: %w", err)
	}

	// 2. Migrate state if needed
	if err := migrateState(planningDir); err != nil {
		return fmt.Errorf("state migration failed: %w", err)
	}

	// 3. Migrate plans in each phase directory
	phasesDir := filepath.Join(planningDir, "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return fmt.Errorf("cannot read phases directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		phaseDir := filepath.Join(phasesDir, entry.Name())
		if err := migratePhasePlans(phaseDir); err != nil {
			return fmt.Errorf("phase %s migration failed: %w", entry.Name(), err)
		}
	}

	return nil
}

// migrateRoadmap converts ROADMAP.md to roadmap.json
func migrateRoadmap(planningDir string) error {
	roadmapJSON := filepath.Join(planningDir, "roadmap.json")

	// Skip if JSON already exists
	if _, err := os.Stat(roadmapJSON); err == nil {
		return nil
	}

	roadmapMD := filepath.Join(planningDir, "ROADMAP.md")
	content, err := os.ReadFile(roadmapMD)
	if err != nil {
		return fmt.Errorf("cannot read ROADMAP.md: %w", err)
	}

	text := string(content)

	// Parse project name from first heading
	projectName := "Unknown Project"
	if matches := regexp.MustCompile(`(?m)^# (.+)$`).FindStringSubmatch(text); len(matches) >= 2 {
		projectName = strings.TrimSpace(matches[1])
	}

	// Parse overview (text before first phase section)
	overview := ""
	if idx := strings.Index(text, "## Phase"); idx > 0 {
		overview = strings.TrimSpace(text[:idx])
		// Remove the title line
		if lines := strings.Split(overview, "\n"); len(lines) > 1 {
			overview = strings.Join(lines[1:], "\n")
		}
	}

	// Parse phases from markdown headers
	var phases []types.Phase
	phasePattern := regexp.MustCompile(`(?m)^### Phase (\d+): (.+)$`)
	phaseMatches := phasePattern.FindAllStringSubmatch(text, -1)

	for _, match := range phaseMatches {
		if len(match) < 3 {
			continue
		}

		phaseNum, _ := strconv.Atoi(match[1])
		phaseName := strings.TrimSpace(match[2])

		// Extract goal (next paragraph after phase header)
		phaseIdx := strings.Index(text, match[0])
		goalText := text[phaseIdx:]
		goalLines := strings.Split(goalText, "\n")
		goal := ""
		for i := 1; i < len(goalLines); i++ {
			line := strings.TrimSpace(goalLines[i])
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") {
				goal = line
				break
			}
		}

		// Find plans for this phase by looking for checkboxes
		planPattern := regexp.MustCompile(fmt.Sprintf(`- \[[x ]\] (%02d-\d+):`, phaseNum))
		planMatches := planPattern.FindAllStringSubmatch(text, -1)
		var planIDs []string
		for _, pm := range planMatches {
			if len(pm) >= 2 {
				planIDs = append(planIDs, pm[1])
			}
		}

		// Determine status based on checkboxes
		status := types.StatusPending
		if len(planIDs) > 0 {
			completedCount := 0
			for _, pm := range planPattern.FindAllStringSubmatch(text, -1) {
				if strings.Contains(pm[0], "[x]") {
					completedCount++
				}
			}
			if completedCount == len(planIDs) {
				status = types.StatusComplete
			} else if completedCount > 0 {
				status = types.StatusInProgress
			}
		}

		phases = append(phases, types.Phase{
			Number: phaseNum,
			Name:   phaseName,
			Goal:   goal,
			Status: status,
			Plans:  planIDs,
		})
	}

	roadmap := &types.Roadmap{
		Version:     "1.0",
		ProjectName: projectName,
		Overview:    overview,
		Phases:      phases,
	}

	return SaveRoadmapJSON(planningDir, roadmap)
}

// migrateState converts STATE.md to state.json
func migrateState(planningDir string) error {
	stateJSON := filepath.Join(planningDir, "state.json")

	// Skip if JSON already exists
	if _, err := os.Stat(stateJSON); err == nil {
		return nil
	}

	stateMD := filepath.Join(planningDir, "STATE.md")
	content, err := os.ReadFile(stateMD)
	if err != nil {
		return fmt.Errorf("cannot read STATE.md: %w", err)
	}

	text := string(content)

	// Parse current phase from "Phase: X of Y" line
	currentPhase := 1
	if matches := regexp.MustCompile(`(?m)^Phase:\s*(\d+)\s*of\s*\d+`).FindStringSubmatch(text); len(matches) >= 2 {
		currentPhase, _ = strconv.Atoi(matches[1])
	}

	projectState := &types.ProjectState{
		Version:      "1.0",
		CurrentPhase: currentPhase,
		LastUpdated:  time.Now(),
	}

	return SaveStateJSON(planningDir, projectState)
}

// migratePhasePlans converts all PLAN.md files in a phase directory to JSON
func migratePhasePlans(phaseDir string) error {
	entries, err := os.ReadDir(phaseDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, "-PLAN.md") {
			continue
		}

		planPath := filepath.Join(phaseDir, name)

		// Extract plan number from filename (e.g., "02-04-PLAN.md" -> "02-04")
		planID := strings.TrimSuffix(name, "-PLAN.md")
		parts := strings.Split(planID, "-")
		if len(parts) < 2 {
			continue
		}

		phaseID := parts[0]
		planNum := parts[1]

		// Check if JSON already exists
		jsonPath := filepath.Join(phaseDir, fmt.Sprintf("%s-%s.json", phaseID, planNum))
		if _, err := os.Stat(jsonPath); err == nil {
			continue // Skip if already migrated
		}

		// Migrate this plan
		if err := migratePlan(planPath, jsonPath, phaseID, planNum); err != nil {
			return fmt.Errorf("plan %s migration failed: %w", name, err)
		}
	}

	return nil
}

// migratePlan converts a single PLAN.md file to JSON format
func migratePlan(planPath, jsonPath, phaseID, planNum string) error {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return err
	}

	text := string(content)

	// Parse frontmatter
	var status types.Status = types.StatusPending
	if strings.Contains(text, "status: complete") {
		status = types.StatusComplete
	} else if strings.Contains(text, "status: in_progress") {
		status = types.StatusInProgress
	}

	// Parse objective from <objective> tag
	objective := ""
	if matches := regexp.MustCompile(`(?s)<objective>\s*(.+?)\s*</objective>`).FindStringSubmatch(text); len(matches) >= 2 {
		objective = strings.TrimSpace(matches[1])
		// Remove "Purpose:" and "Output:" lines for cleaner objective
		lines := strings.Split(objective, "\n")
		var cleanLines []string
		for _, line := range lines {
			if !strings.HasPrefix(line, "Purpose:") && !strings.HasPrefix(line, "Output:") {
				cleanLines = append(cleanLines, line)
			}
		}
		objective = strings.TrimSpace(strings.Join(cleanLines, "\n"))
	}

	// Parse tasks from <task> tags
	var tasks []types.Task
	taskPattern := regexp.MustCompile(`(?s)<task[^>]*type="([^"]*)"[^>]*>\s*<name>([^<]+)</name>\s*<files>([^<]*)</files>\s*<action>(.*?)</action>`)
	taskMatches := taskPattern.FindAllStringSubmatch(text, -1)

	for i, match := range taskMatches {
		if len(match) < 5 {
			continue
		}

		taskType := types.TaskTypeAuto
		if match[1] == "manual" {
			taskType = types.TaskTypeManual
		}

		taskName := strings.TrimSpace(match[2])
		filesStr := strings.TrimSpace(match[3])
		action := strings.TrimSpace(match[4])

		var files []string
		if filesStr != "" {
			files = strings.Split(filesStr, ",")
			for j := range files {
				files[j] = strings.TrimSpace(files[j])
			}
		}

		tasks = append(tasks, types.Task{
			ID:     fmt.Sprintf("task-%d", i+1),
			Name:   taskName,
			Type:   taskType,
			Files:  files,
			Action: action,
			Status: types.StatusPending,
		})
	}

	// Parse verification from <verification> section
	var verification []string
	if matches := regexp.MustCompile(`(?s)<verification>(.*?)</verification>`).FindStringSubmatch(text); len(matches) >= 2 {
		verifyText := strings.TrimSpace(matches[1])
		// Extract checkbox items
		checkPattern := regexp.MustCompile(`(?m)^- \[ \] (.+)$`)
		checkMatches := checkPattern.FindAllStringSubmatch(verifyText, -1)
		for _, cm := range checkMatches {
			if len(cm) >= 2 {
				verification = append(verification, strings.TrimSpace(cm[1]))
			}
		}
	}

	plan := &types.Plan{
		Phase:        fmt.Sprintf("%s-phase-name", phaseID), // Simplified, could parse from directory
		PlanNumber:   planNum,
		Status:       status,
		Objective:    objective,
		Tasks:        tasks,
		Verification: verification,
		CreatedAt:    time.Now(),
	}

	return SavePlanJSON(jsonPath, plan)
}
