// Package task provides task management for the TUI coding agent manager.
package task

import "time"

// SubTaskStatus represents the lifecycle state of a sub-task.
type SubTaskStatus string

const (
	SubTaskPlanned   SubTaskStatus = "planned"
	SubTaskCoding    SubTaskStatus = "coding"
	SubTaskVerifying SubTaskStatus = "verifying"
	SubTaskStuck     SubTaskStatus = "stuck"
	SubTaskDone      SubTaskStatus = "done"
	SubTaskFailed    SubTaskStatus = "failed"
)

// IsTerminal returns true if the sub-task is in a terminal state.
func (s SubTaskStatus) IsTerminal() bool {
	return s == SubTaskDone || s == SubTaskFailed || s == SubTaskStuck
}

// IsActive returns true if the sub-task can accept agent operations.
func (s SubTaskStatus) IsActive() bool {
	return s == SubTaskPlanned || s == SubTaskCoding || s == SubTaskVerifying
}

// Phase constants for the two-phase execution model.
const (
	PhaseCoding      = "coding"
	PhaseVerifying   = "verifying"
)

// SubTask represents a single unit of work within a Task.
// Each SubTask has its own directory and two-phase agent execution (coding + verification).
type SubTask struct {
	ID          string        `json:"id"`
	TaskID      string        `json:"taskId"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Directory   string        `json:"directory"`
	Status      SubTaskStatus `json:"status"`
	OrderIndex  int           `json:"orderIndex"`

	// ACP session IDs for each phase
	CodingSessionID      string `json:"codingSessionId,omitempty"`
	VerificationSessionID string `json:"verificationSessionId,omitempty"`

	// Phase timestamps
	CodingStartedAt         *time.Time `json:"codingStartedAt,omitempty"`
	CodingCompletedAt       *time.Time `json:"codingCompletedAt,omitempty"`
	VerificationStartedAt   *time.Time `json:"verificationStartedAt,omitempty"`
	VerificationCompletedAt *time.Time `json:"verificationCompletedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SubTaskPlan is used during the planning phase before sub-tasks are created.
type SubTaskPlan struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Directory   string `json:"directory"`
	OrderIndex  int    `json:"orderIndex"`
}

// ValidateTransition checks if a sub-task status transition is valid.
func (st *SubTask) ValidateTransition(newStatus SubTaskStatus) error {
	switch st.Status {
	case SubTaskPlanned:
		if newStatus == SubTaskCoding || newStatus == SubTaskFailed {
			return nil
		}
	case SubTaskCoding:
		if newStatus == SubTaskVerifying || newStatus == SubTaskFailed || newStatus == SubTaskDone || newStatus == SubTaskStuck {
			return nil
		}
	case SubTaskVerifying:
		if newStatus == SubTaskDone || newStatus == SubTaskFailed {
			return nil
		}
	case SubTaskDone, SubTaskFailed:
		return &ErrInvalidSubTaskTransition{From: st.Status, To: newStatus}
	}
	return &ErrInvalidSubTaskTransition{From: st.Status, To: newStatus}
}

// CurrentPhase returns the current phase name for the sub-task.
func (st *SubTask) CurrentPhase() string {
	switch st.Status {
	case SubTaskPlanned:
		return "planned"
	case SubTaskCoding:
		return PhaseCoding
	case SubTaskVerifying:
		return PhaseVerifying
	case SubTaskStuck:
		return "stuck"
	case SubTaskDone:
		return "done"
	case SubTaskFailed:
		return "failed"
	}
	return "unknown"
}

// ErrInvalidSubTaskTransition is returned when a status transition is not allowed.
type ErrInvalidSubTaskTransition struct {
	From SubTaskStatus
	To   SubTaskStatus
}

func (e *ErrInvalidSubTaskTransition) Error() string {
	return "invalid sub-task status transition: " + string(e.From) + " -> " + string(e.To)
}
