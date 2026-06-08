package task

import (
	"testing"
	"time"
)

func TestTask_ValidateTransition(t *testing.T) {
	tests := []struct {
		name        string
		from        TaskStatus
		to          TaskStatus
		expectError bool
	}{
		{"pending to in_progress", TaskPending, TaskInProgress, false},
		{"pending to completed", TaskPending, TaskCompleted, false},
		{"pending to review", TaskPending, TaskReview, false},
		{"pending to blocked", TaskPending, TaskBlocked, false},
		{"pending to cancelled", TaskPending, TaskCancelled, false},
		{"in_progress to review", TaskInProgress, TaskReview, false},
		{"in_progress to completed", TaskInProgress, TaskCompleted, false},
		{"in_progress to blocked", TaskInProgress, TaskBlocked, false},
		{"review to completed", TaskReview, TaskCompleted, false},
		{"review to in_progress", TaskReview, TaskInProgress, false},
		{"blocked to pending", TaskBlocked, TaskPending, false},
		{"blocked to in_progress", TaskBlocked, TaskInProgress, false},
		{"completed to in_progress", TaskCompleted, TaskInProgress, true},
		{"completed to review", TaskCompleted, TaskReview, true},
		{"cancelled to pending", TaskCancelled, TaskPending, true},
		{"cancelled to in_progress", TaskCancelled, TaskInProgress, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Status: tt.from}
			err := task.ValidateTransition(tt.to)
			if tt.expectError && err == nil {
				t.Errorf("expected error for %s -> %s", tt.from, tt.to)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for %s -> %s: %v", tt.from, tt.to, err)
			}
		})
	}
}

func TestTaskStatus_IsTerminal(t *testing.T) {
	if TaskPending.IsTerminal() {
		t.Error("pending should not be terminal")
	}
	if TaskInProgress.IsTerminal() {
		t.Error("in_progress should not be terminal")
	}
	if TaskReview.IsTerminal() {
		t.Error("review should not be terminal")
	}
	if TaskBlocked.IsTerminal() {
		t.Error("blocked should not be terminal")
	}
	if !TaskCompleted.IsTerminal() {
		t.Error("completed should be terminal")
	}
	if !TaskCancelled.IsTerminal() {
		t.Error("cancelled should be terminal")
	}
}

func TestTaskStatus_IsActive(t *testing.T) {
	if !TaskPending.IsActive() {
		t.Error("pending should be active")
	}
	if !TaskInProgress.IsActive() {
		t.Error("in_progress should be active")
	}
	if !TaskReview.IsActive() {
		t.Error("review should be active")
	}
	if TaskBlocked.IsActive() {
		t.Error("blocked should NOT be active")
	}
	if TaskCompleted.IsActive() {
		t.Error("completed should not be active")
	}
}

func TestTask_HasSubTasks(t *testing.T) {
	task := &Task{}
	if task.HasSubTasks() {
		t.Error("empty task should not have sub-tasks")
	}
	task.SubTaskIDs = []string{"st-1", "st-2"}
	if !task.HasSubTasks() {
		t.Error("task with sub-task IDs should report having sub-tasks")
	}
}

func TestSubTask_ValidateTransition(t *testing.T) {
	tests := []struct {
		name        string
		from        SubTaskStatus
		to          SubTaskStatus
		expectError bool
	}{
		{"planned to coding", SubTaskPlanned, SubTaskCoding, false},
		{"planned to failed", SubTaskPlanned, SubTaskFailed, false},
		{"planned to verifying", SubTaskPlanned, SubTaskVerifying, true},
		{"planned to done", SubTaskPlanned, SubTaskDone, true},
		{"coding to verifying", SubTaskCoding, SubTaskVerifying, false},
		{"coding to done", SubTaskCoding, SubTaskDone, false},
		{"coding to failed", SubTaskCoding, SubTaskFailed, false},
		{"coding to planned", SubTaskCoding, SubTaskPlanned, true},
		{"verifying to done", SubTaskVerifying, SubTaskDone, false},
		{"verifying to failed", SubTaskVerifying, SubTaskFailed, false},
		{"verifying to coding", SubTaskVerifying, SubTaskCoding, true},
		{"done to anything", SubTaskDone, SubTaskCoding, true},
		{"failed to anything", SubTaskFailed, SubTaskCoding, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &SubTask{Status: tt.from}
			err := st.ValidateTransition(tt.to)
			if tt.expectError && err == nil {
				t.Errorf("expected error for %s -> %s", tt.from, tt.to)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for %s -> %s: %v", tt.from, tt.to, err)
			}
		})
	}
}

func TestSubTask_CurrentPhase(t *testing.T) {
	tests := []struct {
		status    SubTaskStatus
		wantPhase string
	}{
		{SubTaskPlanned, "planned"},
		{SubTaskCoding, PhaseCoding},
		{SubTaskVerifying, PhaseVerifying},
		{SubTaskDone, "done"},
		{SubTaskFailed, "failed"},
	}

	for _, tt := range tests {
		st := &SubTask{Status: tt.status}
		if got := st.CurrentPhase(); got != tt.wantPhase {
			t.Errorf("CurrentPhase(%s) = %q, want %q", tt.status, got, tt.wantPhase)
		}
	}
}

func TestSubTaskStatus_TerminalAndActive(t *testing.T) {
	if SubTaskPlanned.IsTerminal() {
		t.Error("planned should not be terminal")
	}
	if !SubTaskDone.IsTerminal() {
		t.Error("done should be terminal")
	}
	if !SubTaskFailed.IsTerminal() {
		t.Error("failed should be terminal")
	}

	if !SubTaskPlanned.IsActive() {
		t.Error("planned should be active")
	}
	if !SubTaskCoding.IsActive() {
		t.Error("coding should be active")
	}
	if SubTaskDone.IsActive() {
		t.Error("done should NOT be active")
	}
	if SubTaskFailed.IsActive() {
		t.Error("failed should NOT be active")
	}
}

func TestErrInvalidTransition_Error(t *testing.T) {
	err := &ErrInvalidTransition{From: TaskPending, To: TaskCancelled}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestErrInvalidSubTaskTransition_Error(t *testing.T) {
	err := &ErrInvalidSubTaskTransition{From: SubTaskCoding, To: SubTaskDone}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestBoundDirectory(t *testing.T) {
	now := time.Now()
	d := BoundDirectory{Path: "/test", Label: "test-dir", AddedAt: now}
	if d.Path != "/test" {
		t.Errorf("expected path /test, got %q", d.Path)
	}
	if d.Label != "test-dir" {
		t.Errorf("expected label test-dir, got %q", d.Label)
	}
	if !d.AddedAt.Equal(now) {
		t.Error("AddedAt mismatch")
	}
}

func TestTaskFilter(t *testing.T) {
	f := TaskFilter{
		Status:   TaskPending,
		Priority: PriorityHigh,
		Label:    "urgent",
		Limit:    10,
		Offset:   0,
	}
	if f.Status != TaskPending {
		t.Error("status mismatch")
	}
	if f.Priority != PriorityHigh {
		t.Error("priority mismatch")
	}
	if f.Limit != 10 {
		t.Error("limit mismatch")
	}
}

func TestPriorityConstants(t *testing.T) {
	if PriorityLow != "low" {
		t.Error("PriorityLow mismatch")
	}
	if PriorityMedium != "medium" {
		t.Error("PriorityMedium mismatch")
	}
	if PriorityHigh != "high" {
		t.Error("PriorityHigh mismatch")
	}
}
