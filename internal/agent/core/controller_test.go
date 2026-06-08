package core

import (
	"context"
	"testing"

	"codeg/internal/acp"
	"codeg/internal/agent"
	"codeg/internal/task"
)

// mockRunner implements agent.AgentRunner for testing.
type mockRunner struct {
	startResult *agent.StartResult
	startErr    error
}

func (m *mockRunner) Start(ctx context.Context, cwd, prompt string, role acp.AgentRole) (*agent.StartResult, error) {
	return m.startResult, m.startErr
}

func (m *mockRunner) Resume(ctx context.Context, sessionID string) (*agent.StartResult, error) {
	return m.startResult, m.startErr
}

func (m *mockRunner) Load(ctx context.Context, sessionID string) (*acp.SessionResponse, error) {
	return &acp.SessionResponse{SessionID: sessionID, Status: "active"}, nil
}

func newMockResult(sessionID string) *agent.StartResult {
	ch := make(chan acp.SSEEvent, 1)
	ch <- acp.SSEEvent{Type: acp.EventTurnComplete, Data: map[string]interface{}{}}
	close(ch)
	return &agent.StartResult{
		SessionID: sessionID,
		Events:    ch,
		Cancel:    func() error { return nil },
	}
}

func TestNewAgentController(t *testing.T) {
	runner := &mockRunner{}
	ctrl := NewAgentController(runner)
	if ctrl == nil {
		t.Fatal("expected non-nil controller")
	}
	if ctrl.runner == nil {
		t.Error("expected runner to be set")
	}
}

func TestStartSubTaskCoding(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-coding-001"),
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{
		ID:        "st-1",
		TaskID:    "task-1",
		Title:     "Test sub-task",
		Directory: "/project/test",
	}
	taskObj := &task.Task{
		ID:    "task-1",
		Title: "Test task",
	}

	handle, err := ctrl.StartSubTaskCoding(context.Background(), st, taskObj)
	if err != nil {
		t.Fatalf("StartSubTaskCoding failed: %v", err)
	}
	if handle.SessionID != "ses-coding-001" {
		t.Errorf("expected session ID 'ses-coding-001', got %q", handle.SessionID)
	}
	if handle.Phase != task.PhaseCoding {
		t.Errorf("expected phase 'coding', got %q", handle.Phase)
	}
	if handle.Directory != "/project/test" {
		t.Errorf("expected dir '/project/test', got %q", handle.Directory)
	}
}

func TestStartSubTaskVerification(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-verify-001"),
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{
		ID:        "st-2",
		TaskID:    "task-1",
		Title:     "Verify me",
		Directory: "/project/verify",
	}
	taskObj := &task.Task{
		ID:    "task-1",
		Title: "Test task",
	}

	handle, err := ctrl.StartSubTaskVerification(context.Background(), st, taskObj)
	if err != nil {
		t.Fatalf("StartSubTaskVerification failed: %v", err)
	}
	if handle.Phase != task.PhaseVerifying {
		t.Errorf("expected phase 'verifying', got %q", handle.Phase)
	}
}

func TestAgentController_DuplicateStart(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-dup"),
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{
		ID:        "st-dup",
		TaskID:    "task-1",
		Directory: "/project",
	}
	taskObj := &task.Task{ID: "task-1"}

	// First start should succeed
	_, err := ctrl.StartSubTaskCoding(context.Background(), st, taskObj)
	if err != nil {
		t.Fatalf("first StartSubTaskCoding failed: %v", err)
	}

	// Second start should fail (duplicate)
	_, err = ctrl.StartSubTaskCoding(context.Background(), st, taskObj)
	if err == nil {
		t.Error("expected error for duplicate agent start")
	}
}

func TestCancelAgent(t *testing.T) {
	cancelled := false
	runner := &mockRunner{
		startResult: &agent.StartResult{
			SessionID: "ses-cancel",
			Events:    make(chan acp.SSEEvent),
			Cancel:    func() error { cancelled = true; return nil },
		},
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{ID: "st-cancel", TaskID: "task-1", Directory: "/project"}
	taskObj := &task.Task{ID: "task-1"}

	_, err := ctrl.StartSubTaskCoding(context.Background(), st, taskObj)
	if err != nil {
		t.Fatalf("StartSubTaskCoding failed: %v", err)
	}

	err = ctrl.CancelAgent("st-cancel")
	if err != nil {
		t.Fatalf("CancelAgent failed: %v", err)
	}
	if !cancelled {
		t.Error("expected cancel function to be called")
	}
}

func TestCancelAgent_NotFound(t *testing.T) {
	ctrl := NewAgentController(&mockRunner{})
	err := ctrl.CancelAgent("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestGetHandle(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-get"),
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{ID: "st-get", TaskID: "task-1", Directory: "/project"}
	taskObj := &task.Task{ID: "task-1"}

	_, _ = ctrl.StartSubTaskCoding(context.Background(), st, taskObj)

	handle := ctrl.GetHandle("st-get")
	if handle == nil {
		t.Error("expected non-nil handle")
	}
	if handle.SessionID != "ses-get" {
		t.Errorf("expected session ID 'ses-get', got %q", handle.SessionID)
	}
}

func TestGetHandle_NotFound(t *testing.T) {
	ctrl := NewAgentController(&mockRunner{})
	handle := ctrl.GetHandle("nonexistent")
	if handle != nil {
		t.Error("expected nil handle for nonexistent agent")
	}
}

func TestRunningSubTasks(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-running"),
	}
	ctrl := NewAgentController(runner)

	if len(ctrl.RunningSubTasks()) != 0 {
		t.Error("expected 0 running sub-tasks initially")
	}

	st := &task.SubTask{ID: "st-run", TaskID: "task-1", Directory: "/project"}
	taskObj := &task.Task{ID: "task-1"}
	_, _ = ctrl.StartSubTaskCoding(context.Background(), st, taskObj)

	running := ctrl.RunningSubTasks()
	if len(running) != 1 {
		t.Errorf("expected 1 running sub-task, got %d", len(running))
	}
}

func TestRemoveHandle(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-remove"),
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{ID: "st-remove", TaskID: "task-1", Directory: "/project"}
	taskObj := &task.Task{ID: "task-1"}
	_, _ = ctrl.StartSubTaskCoding(context.Background(), st, taskObj)

	ctrl.RemoveHandle("st-remove")
	if ctrl.IsRunning("st-remove") {
		t.Error("expected sub-task to not be running after remove")
	}
}

func TestIsRunning(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-isrunning"),
	}
	ctrl := NewAgentController(runner)

	if ctrl.IsRunning("st-test") {
		t.Error("expected not running initially")
	}

	st := &task.SubTask{ID: "st-test", TaskID: "task-1", Directory: "/project"}
	taskObj := &task.Task{ID: "task-1"}
	_, _ = ctrl.StartSubTaskCoding(context.Background(), st, taskObj)

	if !ctrl.IsRunning("st-test") {
		t.Error("expected sub-task to be running")
	}
}

func TestStartSubTaskCoding_NoDirectory(t *testing.T) {
	runner := &mockRunner{}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{
		ID:     "st-nodir",
		TaskID: "task-1",
	}
	taskObj := &task.Task{
		ID:        "task-1",
		BoundDirs: nil,
	}

	_, err := ctrl.StartSubTaskCoding(context.Background(), st, taskObj)
	if err == nil {
		t.Error("expected error for sub-task with no directory")
	}
}

func TestStartSubTaskCoding_FallbackToTaskDir(t *testing.T) {
	runner := &mockRunner{
		startResult: newMockResult("ses-fallback"),
	}
	ctrl := NewAgentController(runner)

	st := &task.SubTask{
		ID:     "st-fallback",
		TaskID: "task-1",
	}
	taskObj := &task.Task{
		ID: "task-1",
		BoundDirs: []task.BoundDirectory{
			{Path: "/fallback/dir", Label: "main"},
		},
	}

	handle, err := ctrl.StartSubTaskCoding(context.Background(), st, taskObj)
	if err != nil {
		t.Fatalf("StartSubTaskCoding failed: %v", err)
	}
	if handle.Directory != "/fallback/dir" {
		t.Errorf("expected fallback dir '/fallback/dir', got %q", handle.Directory)
	}
}

func TestBuildCodingPrompt(t *testing.T) {
	st := &task.SubTask{
		Title:       "Auth middleware",
		Description: "Implement JWT auth",
	}
	taskObj := &task.Task{
		Title: "Add auth system",
	}

	prompt := buildCodingPrompt(st, taskObj)
	if prompt == "" {
		t.Error("expected non-empty coding prompt")
	}
	if !contains(prompt, "Auth middleware") {
		t.Error("prompt should contain sub-task title")
	}
	if !contains(prompt, "Add auth system") {
		t.Error("prompt should contain task title")
	}
}

func TestBuildVerificationPrompt(t *testing.T) {
	st := &task.SubTask{
		Title:       "Auth middleware",
		Description: "Implement JWT auth",
	}
	taskObj := &task.Task{
		Title: "Add auth system",
	}

	prompt := buildVerificationPrompt(st, taskObj)
	if prompt == "" {
		t.Error("expected non-empty verification prompt")
	}
	if !contains(prompt, "VERIFICATION PASSED") {
		t.Error("prompt should contain VERIFICATION PASSED instruction")
	}
	if !contains(prompt, "VERIFICATION FAILED") {
		t.Error("prompt should contain VERIFICATION FAILED instruction")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
