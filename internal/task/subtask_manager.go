// Package task provides task management for the TUI coding agent manager.
package task

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SubTaskManager manages the lifecycle of sub-tasks within a task.
type SubTaskManager struct {
	store SubTaskStore
	mu    sync.RWMutex
}

// NewSubTaskManager creates a new sub-task manager.
func NewSubTaskManager(store SubTaskStore) *SubTaskManager {
	return &SubTaskManager{store: store}
}

// CreateFromPlans creates sub-tasks from a list of plans.
func (m *SubTaskManager) CreateFromPlans(ctx context.Context, taskID string, plans []SubTaskPlan) ([]*SubTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var subTasks []*SubTask
	for i, plan := range plans {
		if plan.OrderIndex == 0 {
			plan.OrderIndex = i + 1
		}
		st := &SubTask{
			TaskID:      taskID,
			Title:       plan.Title,
			Description: plan.Description,
			Directory:   plan.Directory,
			Status:      SubTaskPlanned,
			OrderIndex:  plan.OrderIndex,
		}
		if err := m.store.Create(ctx, st); err != nil {
			return nil, fmt.Errorf("failed to create sub-task %q: %w", plan.Title, err)
		}
		subTasks = append(subTasks, st)
	}
	return subTasks, nil
}

// Get retrieves a sub-task by ID.
func (m *SubTaskManager) Get(id string) (*SubTask, error) {
	return m.store.Get(context.Background(), id)
}

// ListByTask returns all sub-tasks for a task.
func (m *SubTaskManager) ListByTask(taskID string) ([]*SubTask, error) {
	return m.store.ListByTask(context.Background(), taskID)
}

// Update saves changes to a sub-task.
func (m *SubTaskManager) Update(st *SubTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Update(context.Background(), st)
}

// Delete removes a sub-task.
func (m *SubTaskManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Delete(context.Background(), id)
}

// TransitionStatus changes a sub-task's status with validation.
func (m *SubTaskManager) TransitionStatus(ctx context.Context, id string, newStatus SubTaskStatus) (*SubTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := st.ValidateTransition(newStatus); err != nil {
		return nil, err
	}

	now := time.Now()

	// Set phase timestamps based on the transition
	switch newStatus {
	case SubTaskCoding:
		st.CodingStartedAt = &now
	case SubTaskVerifying:
		st.CodingCompletedAt = &now
		st.VerificationStartedAt = &now
	case SubTaskDone:
		st.VerificationCompletedAt = &now
		if st.VerificationStartedAt == nil {
			st.VerificationStartedAt = &now
		}
		if st.CodingCompletedAt == nil {
			st.CodingCompletedAt = &now
		}
	case SubTaskFailed:
		// Keep existing timestamps
	}

	st.Status = newStatus

	if err := m.store.Update(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// StartCoding transitions a sub-task to the coding phase.
func (m *SubTaskManager) StartCoding(ctx context.Context, id string) (*SubTask, error) {
	return m.TransitionStatus(ctx, id, SubTaskCoding)
}

// StartVerifying transitions a sub-task to the verification phase.
func (m *SubTaskManager) StartVerifying(ctx context.Context, id string) (*SubTask, error) {
	return m.TransitionStatus(ctx, id, SubTaskVerifying)
}

// MarkDone marks a sub-task as completed.
func (m *SubTaskManager) MarkDone(ctx context.Context, id string) (*SubTask, error) {
	return m.TransitionStatus(ctx, id, SubTaskDone)
}

// MarkFailed marks a sub-task as failed.
func (m *SubTaskManager) MarkFailed(ctx context.Context, id string) (*SubTask, error) {
	return m.TransitionStatus(ctx, id, SubTaskFailed)
}

// MarkStuck marks a sub-task as stuck (agent couldn't complete).
func (m *SubTaskManager) MarkStuck(ctx context.Context, id string) (*SubTask, error) {
	return m.TransitionStatus(ctx, id, SubTaskStuck)
}

// LinkSession links an ACP session to a sub-task phase.
func (m *SubTaskManager) LinkSession(ctx context.Context, subTaskID, phase, sessionID string) error {
	return m.store.SetSessionID(ctx, subTaskID, phase, sessionID)
}

// NextPlanned returns the next planned sub-task in order, or nil.
func (m *SubTaskManager) NextPlanned(taskID string) (*SubTask, error) {
	subTasks, err := m.ListByTask(taskID)
	if err != nil {
		return nil, err
	}
	for _, st := range subTasks {
		if st.Status == SubTaskPlanned {
			return st, nil
		}
	}
	return nil, nil
}

// AllDone returns true if all sub-tasks for a task are in a terminal state.
func (m *SubTaskManager) AllDone(taskID string) (bool, error) {
	subTasks, err := m.ListByTask(taskID)
	if err != nil {
		return false, err
	}
	if len(subTasks) == 0 {
		return true, nil
	}
	for _, st := range subTasks {
		if !st.Status.IsTerminal() {
			return false, nil
		}
	}
	return true, nil
}

// CountByStatus returns counts of sub-tasks grouped by status.
func (m *SubTaskManager) CountByStatus(taskID string) (map[SubTaskStatus]int, error) {
	subTasks, err := m.ListByTask(taskID)
	if err != nil {
		return nil, err
	}
	counts := make(map[SubTaskStatus]int)
	for _, st := range subTasks {
		counts[st.Status]++
	}
	return counts, nil
}

// Store returns the underlying sub-task store.
func (m *SubTaskManager) Store() SubTaskStore {
	return m.store
}
