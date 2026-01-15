package state

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// State represents the current project state parsed from STATE.md
type State struct {
	ProjectName    string
	CurrentPhase   int
	TotalPhases    int
	CurrentPlan    int
	TotalPlans     int
	Status         string
	LastActivity   string
	PlansCompleted int
	PlansFailed    int
}

// Phase represents a phase directory
type Phase struct {
	Number      int
	Name        string
	Path        string
	Plans       []Plan
	IsCompleted bool
}

// Plan represents a PLAN.md file
type Plan struct {
	Number      string // String to support decimal plan numbers like "5.1"
	Name        string
	Path        string
	Type        PlanType
	Status      string
	IsCompleted bool // Has corresponding SUMMARY.md
}

// IsManual returns true if the plan requires manual execution
func (p *Plan) IsManual() bool {
	return p.Type == PlanTypeManual
}

// ParsePlanFrontmatter extracts and validates YAML frontmatter from a plan file
func ParsePlanFrontmatter(path string) (*PlanFrontmatter, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := string(content)
	if !strings.HasPrefix(text, "---") {
		return nil, nil // No frontmatter
	}
	end := strings.Index(text[3:], "---")
	if end == -1 {
		return nil, nil
	}
	yamlContent := text[3 : 3+end]

	var fm PlanFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// LoadState reads STATE.md and parses project state
func LoadState(planningDir string) (*State, error) {
	statePath := filepath.Join(planningDir, "STATE.md")
	content, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read STATE.md: %w", err)
	}

	state := &State{}
	lines := strings.Split(string(content), "\n")

	// Parse key fields from STATE.md
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Project name from first heading
		if strings.HasPrefix(line, "# ") && state.ProjectName == "" {
			state.ProjectName = strings.TrimPrefix(line, "# ")
			state.ProjectName = strings.TrimSuffix(state.ProjectName, " State")
		}

		// Phase: X of Y (Phase Name)
		if strings.HasPrefix(line, "Phase:") {
			re := regexp.MustCompile(`Phase:\s*(\d+)\s*of\s*(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
				state.CurrentPhase, _ = strconv.Atoi(matches[1])
				state.TotalPhases, _ = strconv.Atoi(matches[2])
			}
		}

		// Plan: X of Y
		if strings.HasPrefix(line, "Plan:") {
			re := regexp.MustCompile(`Plan:\s*(\d+)\s*of\s*(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
				state.CurrentPlan, _ = strconv.Atoi(matches[1])
				state.TotalPlans, _ = strconv.Atoi(matches[2])
			}
		}

		// Status: ...
		if strings.HasPrefix(line, "Status:") {
			state.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}

		// Last activity: ...
		if strings.HasPrefix(line, "Last activity:") {
			state.LastActivity = strings.TrimSpace(strings.TrimPrefix(line, "Last activity:"))
		}
	}

	return state, nil
}

// LoadPhases scans .planning/phases/ for phase directories and plans
func LoadPhases(planningDir string) ([]Phase, error) {
	phasesDir := filepath.Join(planningDir, "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read phases directory: %w", err)
	}

	var phases []Phase
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse phase number from directory name (e.g., "01-foundation")
		name := entry.Name()
		phase := Phase{
			Name: name,
			Path: filepath.Join(phasesDir, name),
		}

		// Extract phase number
		re := regexp.MustCompile(`^(\d+)`)
		if matches := re.FindStringSubmatch(name); len(matches) >= 2 {
			phase.Number, _ = strconv.Atoi(matches[1])
		}

		// Load plans within this phase
		phase.Plans, _ = loadPlans(phase.Path)

		// Phase is completed if all plans are completed
		phase.IsCompleted = len(phase.Plans) > 0
		for _, p := range phase.Plans {
			if !p.IsCompleted {
				phase.IsCompleted = false
				break
			}
		}

		phases = append(phases, phase)
	}

	// Sort by phase number
	sort.Slice(phases, func(i, j int) bool {
		return phases[i].Number < phases[j].Number
	})

	return phases, nil
}

func loadPlans(phaseDir string) ([]Plan, error) {
	entries, err := os.ReadDir(phaseDir)
	if err != nil {
		return nil, err
	}

	var plans []Plan
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, "-PLAN.md") {
			continue
		}

		plan := Plan{
			Name: name,
			Path: filepath.Join(phaseDir, name),
		}

		// Parse frontmatter (REQUIRED)
		fm, err := ParsePlanFrontmatter(plan.Path)
		if err != nil {
			return nil, fmt.Errorf("plan %s: %w", plan.Name, err)
		}
		if fm == nil {
			return nil, fmt.Errorf("plan %s: missing frontmatter", plan.Name)
		}
		plan.Type = fm.Type
		plan.Status = fm.Status
		plan.Number = fm.Plan

		// Check if SUMMARY.md exists (plan completed)
		summaryName := strings.Replace(name, "-PLAN.md", "-SUMMARY.md", 1)
		summaryPath := filepath.Join(phaseDir, summaryName)
		if _, err := os.Stat(summaryPath); err == nil {
			plan.IsCompleted = true
		}

		plans = append(plans, plan)
	}

	// Sort by plan number (handles decimals like "5.1"), with manual plans last
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].IsManual() != plans[j].IsManual() {
			return !plans[i].IsManual()
		}
		ni, _ := strconv.ParseFloat(plans[i].Number, 64)
		nj, _ := strconv.ParseFloat(plans[j].Number, 64)
		return ni < nj
	})

	return plans, nil
}

// FindNextPlan finds the next incomplete plan to execute
func FindNextPlan(phases []Phase) (*Phase, *Plan) {
	for i := range phases {
		phase := &phases[i]
		for j := range phase.Plans {
			plan := &phase.Plans[j]
			if !plan.IsCompleted {
				return phase, plan
			}
		}
	}
	return nil, nil
}

// CountPlans returns total and completed plan counts
// Excludes special plans (XX-00-decisions and XX-99+-verification) to match IsPhaseComplete behavior
func CountPlans(phases []Phase) (total, completed int) {
	for _, phase := range phases {
		for _, plan := range phase.Plans {
			// Skip special plans (decisions XX-00 and verification XX-99+)
			num, _ := strconv.ParseFloat(plan.Number, 64)
			if num == 0 || num >= 99 {
				continue
			}
			total++
			if plan.IsCompleted {
				completed++
			}
		}
	}
	return total, completed
}

// UpdateStateFile updates STATE.md with current progress
func UpdateStateFile(planningDir string, phases []Phase) error {
	statePath := filepath.Join(planningDir, "STATE.md")

	// Read current content
	content, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("cannot read STATE.md: %w", err)
	}

	// Calculate progress
	total, completed := CountPlans(phases)
	percentage := 0
	if total > 0 {
		percentage = (completed * 100) / total
	}

	// Build completion list (e.g., "01-01, 01-02, 01-05.1")
	var completedList []string
	for _, phase := range phases {
		for _, plan := range phase.Plans {
			if plan.IsCompleted {
				completedList = append(completedList,
					fmt.Sprintf("%02d-%s", phase.Number, plan.Number))
			}
		}
	}

	// Update Plan line
	var planLine string
	if completed == 0 {
		planLine = "Plan: Not started"
	} else {
		planLine = fmt.Sprintf("Plan: %d of %d complete (%s)",
			completed, total, strings.Join(completedList, ", "))
	}

	// Update Status line
	nextPhase, nextPlan := FindNextPlan(phases)
	var statusLine string
	if nextPlan != nil {
		statusLine = fmt.Sprintf("Status: In progress - ready for %02d-%s",
			nextPhase.Number, nextPlan.Number)
	} else {
		statusLine = "Status: All plans complete"
	}

	// Update Progress bar (10 chars)
	filled := (percentage * 10) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
	progressLine := fmt.Sprintf("Progress: %s %d%%", bar, percentage)

	// Update Last activity
	today := time.Now().Format("2006-01-02")
	activityLine := fmt.Sprintf("Last activity: %s — Plan completed", today)

	// Update Session Continuity section
	sessionLine := fmt.Sprintf("Last session: %s", today)
	var stoppedLine, resumeLine string
	if nextPlan != nil && len(completedList) > 0 {
		stoppedLine = fmt.Sprintf("Stopped at: Plan %s complete, ready for next",
			completedList[len(completedList)-1])
		resumeLine = fmt.Sprintf("Resume file: %s", nextPlan.Path)
	} else if nextPlan == nil {
		stoppedLine = "Stopped at: All plans complete"
		resumeLine = "Resume file: None"
	} else {
		stoppedLine = "Stopped at: Starting"
		resumeLine = fmt.Sprintf("Resume file: %s", nextPlan.Path)
	}

	// Apply updates using regex replacements
	text := string(content)
	text = regexp.MustCompile(`(?m)^Plan:.*$`).ReplaceAllString(text, planLine)
	text = regexp.MustCompile(`(?m)^Status:.*$`).ReplaceAllString(text, statusLine)
	text = regexp.MustCompile(`(?m)^Progress:.*$`).ReplaceAllString(text, progressLine)
	text = regexp.MustCompile(`(?m)^Last activity:.*$`).ReplaceAllString(text, activityLine)
	text = regexp.MustCompile(`(?m)^Last session:.*$`).ReplaceAllString(text, sessionLine)
	text = regexp.MustCompile(`(?m)^Stopped at:.*$`).ReplaceAllString(text, stoppedLine)
	text = regexp.MustCompile(`(?m)^Resume file:.*$`).ReplaceAllString(text, resumeLine)

	return os.WriteFile(statePath, []byte(text), 0644)
}

// UpdateRoadmap updates ROADMAP.md checkboxes and summary table
func UpdateRoadmap(planningDir string, phases []Phase) error {
	roadmapPath := filepath.Join(planningDir, "ROADMAP.md")

	content, err := os.ReadFile(roadmapPath)
	if err != nil {
		return fmt.Errorf("cannot read ROADMAP.md: %w", err)
	}

	text := string(content)

	// 1. Update plan checkboxes (- [ ] XX-YY: → - [x] XX-YY: ✅)
	for _, phase := range phases {
		for _, plan := range phase.Plans {
			planId := fmt.Sprintf("%02d-%s", phase.Number, plan.Number)
			if plan.IsCompleted {
				// Match patterns like "- [ ] 01-01:" and replace with "- [x] 01-01: ... ✅"
				pattern := regexp.MustCompile(`- \[ \] ` + planId + `:([^\n]+)`)
				text = pattern.ReplaceAllString(text, "- [x] "+planId+":$1 ✅")
			}
		}
	}

	// 2. Update summary table row for each phase
	// Format: | 1. Feature Verification | 3/10 | In progress | - |
	for _, phase := range phases {
		totalPlans := len(phase.Plans)
		completedPlans := 0
		for _, p := range phase.Plans {
			if p.IsCompleted {
				completedPlans++
			}
		}

		status := "Not started"
		if completedPlans > 0 && completedPlans < totalPlans {
			status = "In progress"
		} else if completedPlans == totalPlans && totalPlans > 0 {
			status = "Complete"
		}

		// Update the row: | X. Phase Name | Y/Z | Status | - |
		pattern := regexp.MustCompile(
			fmt.Sprintf(`\| %d\. ([^|]+)\| \d+/\d+ \| [^|]+ \|`, phase.Number))
		replacement := fmt.Sprintf("| %d. $1| %d/%d | %s |",
			phase.Number, completedPlans, totalPlans, status)
		text = pattern.ReplaceAllString(text, replacement)
	}

	return os.WriteFile(roadmapPath, []byte(text), 0644)
}
