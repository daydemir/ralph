package types

import (
	"testing"
	"time"
)

func TestPhaseValidate(t *testing.T) {
	tests := []struct {
		name    string
		phase   Phase
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid phase",
			phase: Phase{
				Number: 1,
				Name:   "Critical Bug Fixes",
				Goal:   "Fix all critical bugs",
				Status: StatusPending,
			},
			wantErr: false,
		},
		{
			name: "valid phase with empty status defaults to pending",
			phase: Phase{
				Number: 1,
				Name:   "Critical Bug Fixes",
				Goal:   "Fix all critical bugs",
				Status: "",
			},
			wantErr: false,
		},
		{
			name: "missing number (zero)",
			phase: Phase{
				Number: 0,
				Name:   "Critical Bug Fixes",
				Goal:   "Fix all critical bugs",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "phase.number: must be positive",
		},
		{
			name: "negative number",
			phase: Phase{
				Number: -1,
				Name:   "Critical Bug Fixes",
				Goal:   "Fix all critical bugs",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "phase.number: must be positive",
		},
		{
			name: "missing name",
			phase: Phase{
				Number: 1,
				Name:   "",
				Goal:   "Fix all critical bugs",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "phase.name: field is required",
		},
		{
			name: "missing goal",
			phase: Phase{
				Number: 1,
				Name:   "Critical Bug Fixes",
				Goal:   "",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "phase.goal: field is required",
		},
		{
			name: "invalid status",
			phase: Phase{
				Number: 1,
				Name:   "Critical Bug Fixes",
				Goal:   "Fix all critical bugs",
				Status: Status("invalid_status"),
			},
			wantErr: true,
			errMsg:  "phase.status: invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.phase.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Phase.Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Phase.Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Phase.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestPlanValidate(t *testing.T) {
	validTime := time.Now()
	validTask := Task{
		ID:     "task-1",
		Name:   "Test Task",
		Type:   TaskTypeAuto,
		Action: "Do something",
		Verify: "Check it works",
		Done:   "It is done",
		Status: StatusPending,
	}

	tests := []struct {
		name    string
		plan    Plan
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid plan",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks:      []Task{validTask},
				CreatedAt:  validTime,
			},
			wantErr: false,
		},
		{
			name: "valid plan with empty status defaults to pending",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     "",
				Objective:  "Fix critical bugs",
				Tasks:      []Task{validTask},
				CreatedAt:  validTime,
			},
			wantErr: false,
		},
		{
			name: "missing phase",
			plan: Plan{
				Phase:      "",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks:      []Task{validTask},
				CreatedAt:  validTime,
			},
			wantErr: true,
			errMsg:  "plan.phase: field is required",
		},
		{
			name: "missing plan_number",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks:      []Task{validTask},
				CreatedAt:  validTime,
			},
			wantErr: true,
			errMsg:  "plan.plan_number: field is required",
		},
		{
			name: "invalid status",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     Status("bad_status"),
				Objective:  "Fix critical bugs",
				Tasks:      []Task{validTask},
				CreatedAt:  validTime,
			},
			wantErr: true,
			errMsg:  "plan.status: invalid value",
		},
		{
			name: "missing objective",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "",
				Tasks:      []Task{validTask},
				CreatedAt:  validTime,
			},
			wantErr: true,
			errMsg:  "plan.objective: field is required",
		},
		{
			name: "missing tasks",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks:      []Task{},
				CreatedAt:  validTime,
			},
			wantErr: true,
			errMsg:  "plan.tasks: at least one task is required",
		},
		{
			name: "nil tasks",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks:      nil,
				CreatedAt:  validTime,
			},
			wantErr: true,
			errMsg:  "plan.tasks: at least one task is required",
		},
		{
			name: "missing created_at",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks:      []Task{validTask},
				CreatedAt:  time.Time{},
			},
			wantErr: true,
			errMsg:  "plan.created_at: field is required",
		},
		{
			name: "invalid task in plan",
			plan: Plan{
				Phase:      "01-critical-fixes",
				PlanNumber: "01",
				Status:     StatusPending,
				Objective:  "Fix critical bugs",
				Tasks: []Task{{
					ID:     "",
					Name:   "Test Task",
					Type:   TaskTypeAuto,
					Action: "Do something",
					Verify: "Check it works",
					Done:   "It is done",
					Status: StatusPending,
				}},
				CreatedAt: validTime,
			},
			wantErr: true,
			errMsg:  "plan.tasks[0]:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plan.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Plan.Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Plan.Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Plan.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestTaskValidate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid auto task",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: false,
		},
		{
			name: "valid manual task",
			task: Task{
				ID:     "task-2",
				Name:   "Manual Task",
				Type:   TaskTypeManual,
				Action: "User does something",
				Done:   "User confirmed it works",
				Status: StatusPending,
			},
			wantErr: false,
		},
		{
			name: "valid task with empty type defaults to auto",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   "",
				Action: "Do something",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: false,
		},
		{
			name: "valid task with empty status defaults to pending",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: "",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			task: Task{
				ID:     "",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "task.id: field is required",
		},
		{
			name: "missing name",
			task: Task{
				ID:     "task-1",
				Name:   "",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "task.name: field is required",
		},
		{
			name: "invalid type",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskType("invalid_type"),
				Action: "Do something",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "task.type: invalid value",
		},
		{
			name: "missing action",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "task.action: field is required",
		},
		{
			name: "missing done",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "Check it works",
				Done:   "",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "task.done: field is required",
		},
		{
			name: "missing verify for auto task",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "",
				Done:   "Task is complete",
				Status: StatusPending,
			},
			wantErr: true,
			errMsg:  "task.verify: field is required for auto tasks",
		},
		{
			name: "invalid status",
			task: Task{
				ID:     "task-1",
				Name:   "Test Task",
				Type:   TaskTypeAuto,
				Action: "Do something",
				Verify: "Check it works",
				Done:   "Task is complete",
				Status: Status("bad_status"),
			},
			wantErr: true,
			errMsg:  "task.status: invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Task.Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Task.Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Task.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestPlanIsManualAndGetType(t *testing.T) {
	autoTask := Task{
		ID:     "task-1",
		Name:   "Auto Task",
		Type:   TaskTypeAuto,
		Action: "Do something automatically",
		Verify: "Check it works",
		Done:   "Task is complete",
		Status: StatusPending,
	}
	manualTask := Task{
		ID:     "task-2",
		Name:   "Manual Task",
		Type:   TaskTypeManual,
		Action: "User does something",
		Done:   "User confirmed it works",
		Status: StatusPending,
	}

	tests := []struct {
		name           string
		tasks          []Task
		wantIsManual   bool
		wantGetType    string
	}{
		{
			name:         "plan with only auto tasks",
			tasks:        []Task{autoTask, autoTask},
			wantIsManual: false,
			wantGetType:  PlanTypeExecute,
		},
		{
			name:         "plan with only manual tasks",
			tasks:        []Task{manualTask, manualTask},
			wantIsManual: true,
			wantGetType:  PlanTypeManual,
		},
		{
			name:         "plan with mixed tasks (manual first)",
			tasks:        []Task{manualTask, autoTask},
			wantIsManual: true,
			wantGetType:  PlanTypeManual,
		},
		{
			name:         "plan with mixed tasks (auto first)",
			tasks:        []Task{autoTask, manualTask},
			wantIsManual: true,
			wantGetType:  PlanTypeManual,
		},
		{
			name:         "plan with single auto task",
			tasks:        []Task{autoTask},
			wantIsManual: false,
			wantGetType:  PlanTypeExecute,
		},
		{
			name:         "plan with single manual task",
			tasks:        []Task{manualTask},
			wantIsManual: true,
			wantGetType:  PlanTypeManual,
		},
		{
			name:         "plan with no tasks",
			tasks:        []Task{},
			wantIsManual: false,
			wantGetType:  PlanTypeExecute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &Plan{Tasks: tt.tasks}

			if got := plan.IsManual(); got != tt.wantIsManual {
				t.Errorf("Plan.IsManual() = %v, want %v", got, tt.wantIsManual)
			}

			if got := plan.GetType(); got != tt.wantGetType {
				t.Errorf("Plan.GetType() = %q, want %q", got, tt.wantGetType)
			}
		})
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
