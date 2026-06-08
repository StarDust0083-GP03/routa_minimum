// Package task provides task management for the TUI coding agent manager.
package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codeg/internal/llm"
)

// TaskManager provides high-level task lifecycle operations.
type TaskManager struct {
	store      TaskStore
	subManager *SubTaskManager
	llm        llm.Client
	mu         sync.RWMutex
}

// NewTaskManager creates a new task manager.
func NewTaskManager(store TaskStore, llmClient llm.Client) *TaskManager {
	return &TaskManager{
		store:      store,
		llm:        llmClient,
		subManager: nil, // set via SetSubTaskManager
	}
}

// SetSubTaskManager wires the sub-task manager for decomposition workflows.
func (m *TaskManager) SetSubTaskManager(sm *SubTaskManager) {
	m.subManager = sm
}

// SubTasks returns the sub-task manager, or nil.
func (m *TaskManager) SubTasks() *SubTaskManager {
	return m.subManager
}

// Create creates a new task with the given title and objective.
func (m *TaskManager) Create(ctx context.Context, title, objective string) (*Task, error) {
	task := &Task{
		Title:     title,
		Objective: objective,
		Status:    TaskPending,
		Priority:  PriorityMedium,
	}
	if err := m.store.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	return task, nil
}

// Get retrieves a task by ID.
func (m *TaskManager) Get(id string) (*Task, error) {
	return m.store.Get(context.Background(), id)
}

// Update saves changes to a task.
func (m *TaskManager) Update(task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Update(context.Background(), task)
}

// Delete removes a task.
func (m *TaskManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Delete(context.Background(), id)
}

// List returns tasks matching the filter.
func (m *TaskManager) List(filter TaskFilter) ([]*Task, error) {
	return m.store.List(context.Background(), filter)
}

// ListAll returns all tasks.
func (m *TaskManager) ListAll() ([]*Task, error) {
	return m.store.List(context.Background(), TaskFilter{})
}

// transitionStatus changes a task's status with validation.
func (m *TaskManager) transitionStatus(ctx context.Context, taskID string, newStatus TaskStatus) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, err := m.store.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if err := task.ValidateTransition(newStatus); err != nil {
		return nil, err
	}

	task.Status = newStatus
	if newStatus == TaskCompleted {
		now := time.Now()
		task.CompletedAt = &now
	}

	if err := m.store.Update(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// StartTask transitions a task to in_progress.
func (m *TaskManager) StartTask(ctx context.Context, taskID string) error {
	if _, err := m.transitionStatus(ctx, taskID, TaskInProgress); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}
	return nil
}

// CompleteTask transitions a task to completed and generates a summary.
func (m *TaskManager) CompleteTask(ctx context.Context, taskID string) error {
	task, err := m.transitionStatus(ctx, taskID, TaskCompleted)
	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	// Generate summary asynchronously
	go func() {
		summary, err := m.GenerateSummary(context.Background(), taskID)
		if err != nil {
			summary = fmt.Sprintf("Summary generation failed: %v", err)
		}
		m.mu.Lock()
		task.Summary = summary
		_ = m.store.Update(context.Background(), task)
		m.mu.Unlock()
	}()

	return nil
}

// CancelTask transitions a task to cancelled.
func (m *TaskManager) CancelTask(ctx context.Context, taskID string) error {
	if _, err := m.transitionStatus(ctx, taskID, TaskCancelled); err != nil {
		return fmt.Errorf("failed to cancel task: %w", err)
	}
	return nil
}

// BlockTask transitions a task to blocked.
func (m *TaskManager) BlockTask(ctx context.Context, taskID string) error {
	if _, err := m.transitionStatus(ctx, taskID, TaskBlocked); err != nil {
		return fmt.Errorf("failed to block task: %w", err)
	}
	return nil
}

// ReviewTask transitions a task to review.
func (m *TaskManager) ReviewTask(ctx context.Context, taskID string) error {
	if _, err := m.transitionStatus(ctx, taskID, TaskReview); err != nil {
		return fmt.Errorf("failed to move task to review: %w", err)
	}
	return nil
}

// BindDirectory adds a directory binding to a task.
func (m *TaskManager) BindDirectory(ctx context.Context, taskID string, path, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, err := m.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	// Check for duplicates
	for _, d := range task.BoundDirs {
		if d.Path == path {
			return fmt.Errorf("directory already bound: %s", path)
		}
	}

	dir := BoundDirectory{Path: path, Label: label, AddedAt: time.Now()}
	if err := m.store.AddDirectory(ctx, taskID, dir); err != nil {
		return err
	}
	return nil
}

// UnbindDirectory removes a directory binding from a task.
func (m *TaskManager) UnbindDirectory(ctx context.Context, taskID string, path string) error {
	return m.store.RemoveDirectory(ctx, taskID, path)
}

// LinkSession links a session to a task.
func (m *TaskManager) LinkSession(ctx context.Context, taskID, sessionID string) error {
	return m.store.AddSession(ctx, taskID, sessionID)
}

// GenerateSummary generates a summary for a completed task using the LLM.
// It requires a SessionHistoryProvider to extract the conversation.
func (m *TaskManager) GenerateSummary(ctx context.Context, taskID string) (string, error) {
	task, err := m.store.Get(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}

	if m.llm == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	// Build a conversation summary from task metadata
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Title))
	sb.WriteString(fmt.Sprintf("Status: %s\n", task.Status))
	sb.WriteString(fmt.Sprintf("Bound directories: %d\n", len(task.BoundDirs)))
	for _, d := range task.BoundDirs {
		sb.WriteString(fmt.Sprintf("  - %s (%s)\n", d.Path, d.Label))
	}
	sb.WriteString(fmt.Sprintf("Sessions: %d\n", len(task.SessionIDs)))

	// Include sub-task info if available
	if m.subManager != nil {
		subTasks, err := m.subManager.ListByTask(taskID)
		if err == nil && len(subTasks) > 0 {
			sb.WriteString(fmt.Sprintf("\nSub-tasks: %d\n", len(subTasks)))
			for _, st := range subTasks {
				sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", st.Status, st.Title, st.Directory))
			}
		}
	}

	return m.llm.Summarize(ctx, task.Objective, sb.String())
}

// GetLLMClient returns the LLM client for planning and summarization.
func (m *TaskManager) GetLLMClient() llm.Client {
	return m.llm
}

// Store returns the underlying task store for direct access.
func (m *TaskManager) Store() TaskStore {
	return m.store
}
