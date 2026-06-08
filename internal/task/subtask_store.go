// Package task provides task management for the TUI coding agent manager.
package task

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SubTaskStore defines the persistence interface for sub-tasks.
type SubTaskStore interface {
	Create(ctx context.Context, st *SubTask) error
	Get(ctx context.Context, id string) (*SubTask, error)
	Update(ctx context.Context, st *SubTask) error
	Delete(ctx context.Context, id string) error
	ListByTask(ctx context.Context, taskID string) ([]*SubTask, error)
	UpdateStatus(ctx context.Context, id string, status SubTaskStatus) error
	SetSessionID(ctx context.Context, id, phase, sessionID string) error
	SetPhaseTimestamp(ctx context.Context, id, phase string, t time.Time) error
}

// SQLiteSubTaskStore implements SubTaskStore using SQLite.
type SQLiteSubTaskStore struct {
	db *sql.DB
}

// NewSQLiteSubTaskStore creates a new SQLite-backed sub-task store.
func NewSQLiteSubTaskStore(db *sql.DB) (*SQLiteSubTaskStore, error) {
	store := &SQLiteSubTaskStore{db: db}
	if err := store.initSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to init sub-task schema: %w", err)
	}
	return store, nil
}

func (s *SQLiteSubTaskStore) initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS subtasks (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		directory TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'planned',
		order_index INTEGER NOT NULL DEFAULT 0,
		coding_session_id TEXT DEFAULT '',
		verification_session_id TEXT DEFAULT '',
		coding_started_at DATETIME,
		coding_completed_at DATETIME,
		verification_started_at DATETIME,
		verification_completed_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_subtasks_task_id ON subtasks(task_id);
	CREATE INDEX IF NOT EXISTS idx_subtasks_status ON subtasks(status);
	CREATE INDEX IF NOT EXISTS idx_subtasks_order ON subtasks(task_id, order_index);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Create inserts a new sub-task.
func (s *SQLiteSubTaskStore) Create(ctx context.Context, st *SubTask) error {
	now := time.Now()
	st.ID = uuid.New().String()
	st.CreatedAt = now
	st.UpdatedAt = now
	if st.Status == "" {
		st.Status = SubTaskPlanned
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subtasks (
			id, task_id, title, description, directory, status, order_index,
			coding_session_id, verification_session_id,
			coding_started_at, coding_completed_at,
			verification_started_at, verification_completed_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		st.ID, st.TaskID, st.Title, st.Description, st.Directory, st.Status, st.OrderIndex,
		st.CodingSessionID, st.VerificationSessionID,
		nullTime(st.CodingStartedAt), nullTime(st.CodingCompletedAt),
		nullTime(st.VerificationStartedAt), nullTime(st.VerificationCompletedAt),
		st.CreatedAt, st.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert sub-task: %w", err)
	}
	return nil
}

// Get retrieves a sub-task by ID.
func (s *SQLiteSubTaskStore) Get(ctx context.Context, id string) (*SubTask, error) {
	st := &SubTask{}
	var (
		codingStartedAt, codingCompletedAt           sql.NullTime
		verificationStartedAt, verificationCompletedAt sql.NullTime
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, title, description, directory, status, order_index,
			coding_session_id, verification_session_id,
			coding_started_at, coding_completed_at,
			verification_started_at, verification_completed_at,
			created_at, updated_at
		FROM subtasks WHERE id = ?
	`, id).Scan(
		&st.ID, &st.TaskID, &st.Title, &st.Description, &st.Directory, &st.Status, &st.OrderIndex,
		&st.CodingSessionID, &st.VerificationSessionID,
		&codingStartedAt, &codingCompletedAt,
		&verificationStartedAt, &verificationCompletedAt,
		&st.CreatedAt, &st.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sub-task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-task: %w", err)
	}

	if codingStartedAt.Valid {
		st.CodingStartedAt = &codingStartedAt.Time
	}
	if codingCompletedAt.Valid {
		st.CodingCompletedAt = &codingCompletedAt.Time
	}
	if verificationStartedAt.Valid {
		st.VerificationStartedAt = &verificationStartedAt.Time
	}
	if verificationCompletedAt.Valid {
		st.VerificationCompletedAt = &verificationCompletedAt.Time
	}

	return st, nil
}

// Update saves changes to an existing sub-task.
func (s *SQLiteSubTaskStore) Update(ctx context.Context, st *SubTask) error {
	st.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		UPDATE subtasks SET
			title=?, description=?, directory=?, status=?, order_index=?,
			coding_session_id=?, verification_session_id=?,
			coding_started_at=?, coding_completed_at=?,
			verification_started_at=?, verification_completed_at=?,
			updated_at=?
		WHERE id=?
	`,
		st.Title, st.Description, st.Directory, st.Status, st.OrderIndex,
		st.CodingSessionID, st.VerificationSessionID,
		nullTime(st.CodingStartedAt), nullTime(st.CodingCompletedAt),
		nullTime(st.VerificationStartedAt), nullTime(st.VerificationCompletedAt),
		st.UpdatedAt, st.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update sub-task: %w", err)
	}
	return nil
}

// Delete removes a sub-task.
func (s *SQLiteSubTaskStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM subtasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete sub-task: %w", err)
	}
	return nil
}

// ListByTask returns all sub-tasks for a task, ordered by order_index.
func (s *SQLiteSubTaskStore) ListByTask(ctx context.Context, taskID string) ([]*SubTask, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, title, description, directory, status, order_index,
			coding_session_id, verification_session_id,
			coding_started_at, coding_completed_at,
			verification_started_at, verification_completed_at,
			created_at, updated_at
		FROM subtasks
		WHERE task_id = ?
		ORDER BY order_index ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sub-tasks: %w", err)
	}
	defer rows.Close()

	var subTasks []*SubTask
	for rows.Next() {
		st := &SubTask{}
		var (
			codingStartedAt, codingCompletedAt           sql.NullTime
			verificationStartedAt, verificationCompletedAt sql.NullTime
		)
		if err := rows.Scan(
			&st.ID, &st.TaskID, &st.Title, &st.Description, &st.Directory, &st.Status, &st.OrderIndex,
			&st.CodingSessionID, &st.VerificationSessionID,
			&codingStartedAt, &codingCompletedAt,
			&verificationStartedAt, &verificationCompletedAt,
			&st.CreatedAt, &st.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan sub-task: %w", err)
		}
		if codingStartedAt.Valid {
			st.CodingStartedAt = &codingStartedAt.Time
		}
		if codingCompletedAt.Valid {
			st.CodingCompletedAt = &codingCompletedAt.Time
		}
		if verificationStartedAt.Valid {
			st.VerificationStartedAt = &verificationStartedAt.Time
		}
		if verificationCompletedAt.Valid {
			st.VerificationCompletedAt = &verificationCompletedAt.Time
		}
		subTasks = append(subTasks, st)
	}
	return subTasks, rows.Err()
}

// UpdateStatus updates only the status field of a sub-task.
func (s *SQLiteSubTaskStore) UpdateStatus(ctx context.Context, id string, status SubTaskStatus) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE subtasks SET status=?, updated_at=? WHERE id=?
	`, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update sub-task status: %w", err)
	}
	return nil
}

// SetSessionID sets a session ID for a specific phase.
func (s *SQLiteSubTaskStore) SetSessionID(ctx context.Context, id, phase, sessionID string) error {
	var column string
	switch phase {
	case PhaseCoding:
		column = "coding_session_id"
	case PhaseVerifying:
		column = "verification_session_id"
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE subtasks SET `+column+`=?, updated_at=? WHERE id=?
	`, sessionID, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to set session ID: %w", err)
	}
	return nil
}

// SetPhaseTimestamp sets a start or completion timestamp for a phase.
func (s *SQLiteSubTaskStore) SetPhaseTimestamp(ctx context.Context, id, phase string, t time.Time) error {
	var column string
	switch {
	case phase == "coding_started":
		column = "coding_started_at"
	case phase == "coding_completed":
		column = "coding_completed_at"
	case phase == "verification_started":
		column = "verification_started_at"
	case phase == "verification_completed":
		column = "verification_completed_at"
	default:
		return fmt.Errorf("unknown phase timestamp: %s", phase)
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE subtasks SET `+column+`=?, updated_at=? WHERE id=?
	`, t, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to set phase timestamp: %w", err)
	}
	return nil
}

// nullTime converts *time.Time to interface{} for SQL NULL handling.
func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
