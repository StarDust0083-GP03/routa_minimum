// Package task provides task management for the TUI coding agent manager.
// Tasks track objectives, bound directories, linked sessions, and lifecycle state.
package task

import "time"

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskReview     TaskStatus = "review"
	TaskCompleted  TaskStatus = "completed"
	TaskBlocked    TaskStatus = "blocked"
	TaskCancelled  TaskStatus = "cancelled"
)

// IsTerminal returns true if the status is a terminal state.
func (s TaskStatus) IsTerminal() bool {
	return s == TaskCompleted || s == TaskCancelled
}

// IsActive returns true if the task can accept agent operations.
func (s TaskStatus) IsActive() bool {
	return s == TaskPending || s == TaskInProgress || s == TaskReview
}

// TaskPriority represents the priority level of a task.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

// BoundDirectory represents a working directory bound to a task.
type BoundDirectory struct {
	Path    string    `json:"path"`
	Label   string    `json:"label"`
	AddedAt time.Time `json:"addedAt"`
}

// Task is the central task model.
type Task struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Objective   string           `json:"objective"`
	Status      TaskStatus       `json:"status"`
	Priority    TaskPriority     `json:"priority"`
	BoundDirs   []BoundDirectory `json:"boundDirs"`
	SessionIDs  []string         `json:"sessionIds"`
	Labels      []string         `json:"labels"`
	Summary     string           `json:"summary"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	CompletedAt *time.Time       `json:"completedAt,omitempty"`
}

// TaskFilter defines criteria for listing tasks.
type TaskFilter struct {
	Status   TaskStatus
	Priority TaskPriority
	Label    string
	Limit    int
	Offset   int
}

// ValidateTransition checks if a status transition is valid.
func (t *Task) ValidateTransition(newStatus TaskStatus) error {
	switch t.Status {
	case TaskPending:
		if newStatus == TaskCompleted || newStatus == TaskReview {
			return nil // can complete directly or move to review
		}
		if newStatus == TaskInProgress || newStatus == TaskBlocked || newStatus == TaskCancelled {
			return nil
		}
	case TaskInProgress:
		if newStatus == TaskReview || newStatus == TaskCompleted || newStatus == TaskBlocked || newStatus == TaskCancelled {
			return nil
		}
	case TaskReview:
		if newStatus == TaskCompleted || newStatus == TaskInProgress || newStatus == TaskBlocked {
			return nil
		}
	case TaskBlocked:
		if newStatus == TaskPending || newStatus == TaskInProgress || newStatus == TaskCancelled {
			return nil
		}
	case TaskCompleted, TaskCancelled:
		return &ErrInvalidTransition{From: t.Status, To: newStatus}
	}
	return &ErrInvalidTransition{From: t.Status, To: newStatus}
}

// ErrInvalidTransition is returned when a status transition is not allowed.
type ErrInvalidTransition struct {
	From TaskStatus
	To   TaskStatus
}

func (e *ErrInvalidTransition) Error() string {
	return "invalid task status transition: " + string(e.From) + " -> " + string(e.To)
}
