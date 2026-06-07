// Package task provides task management for the TUI coding agent manager.
package task

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TaskStore defines the persistence interface for tasks.
type TaskStore interface {
	Create(ctx context.Context, task *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	Update(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter TaskFilter) ([]*Task, error)
	AddDirectory(ctx context.Context, taskID string, dir BoundDirectory) error
	RemoveDirectory(ctx context.Context, taskID string, path string) error
	AddSession(ctx context.Context, taskID, sessionID string) error
	Close() error
}

// SQLiteTaskStore implements TaskStore using SQLite.
type SQLiteTaskStore struct {
	db     *sql.DB
	dbPath string
}

// NewSQLiteTaskStore creates a new SQLite-backed task store.
// If db is provided, it reuses the existing connection; otherwise opens its own.
// This allows sharing a database with the session store.
func NewSQLiteTaskStore(ctx context.Context, dbPath string, existingDB *sql.DB) (*SQLiteTaskStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	var db *sql.DB
	var err error
	if existingDB != nil {
		db = existingDB
	} else {
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}

	store := &SQLiteTaskStore{db: db, dbPath: dbPath}
	if err := store.initSchema(ctx); err != nil {
		if existingDB == nil {
			db.Close()
		}
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteTaskStore) initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		objective TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		priority TEXT NOT NULL DEFAULT 'medium',
		summary TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS task_directories (
		task_id TEXT NOT NULL,
		path TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '',
		added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (task_id, path),
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS task_sessions (
		task_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (task_id, session_id),
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS task_labels (
		task_id TEXT NOT NULL,
		label TEXT NOT NULL,
		PRIMARY KEY (task_id, label),
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
	CREATE INDEX IF NOT EXISTS idx_tasks_updated ON tasks(updated_at DESC);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Create inserts a new task.
func (s *SQLiteTaskStore) Create(ctx context.Context, task *Task) error {
	now := time.Now()
	task.ID = uuid.New().String()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = TaskPending
	}
	if task.Priority == "" {
		task.Priority = PriorityMedium
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (id, title, objective, status, priority, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.Title, task.Objective, task.Status, task.Priority, task.Summary, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert task: %w", err)
	}

	// Insert directories
	for _, d := range task.BoundDirs {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO task_directories (task_id, path, label, added_at)
			VALUES (?, ?, ?, ?)
		`, task.ID, d.Path, d.Label, d.AddedAt)
		if err != nil {
			return fmt.Errorf("failed to insert directory: %w", err)
		}
	}

	// Insert labels
	for _, l := range task.Labels {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO task_labels (task_id, label) VALUES (?, ?)
		`, task.ID, l)
		if err != nil {
			return fmt.Errorf("failed to insert label: %w", err)
		}
	}

	return tx.Commit()
}

// Get retrieves a task by ID with all related data.
func (s *SQLiteTaskStore) Get(ctx context.Context, id string) (*Task, error) {
	task := &Task{}
	var completedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, objective, status, priority, summary, created_at, updated_at, completed_at
		FROM tasks WHERE id = ?
	`, id).Scan(
		&task.ID, &task.Title, &task.Objective, &task.Status, &task.Priority,
		&task.Summary, &task.CreatedAt, &task.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	// Load directories
	dirRows, err := s.db.QueryContext(ctx, `
		SELECT path, label, added_at FROM task_directories WHERE task_id = ? ORDER BY added_at
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get directories: %w", err)
	}
	defer dirRows.Close()
	for dirRows.Next() {
		var d BoundDirectory
		if err := dirRows.Scan(&d.Path, &d.Label, &d.AddedAt); err != nil {
			return nil, fmt.Errorf("failed to scan directory: %w", err)
		}
		task.BoundDirs = append(task.BoundDirs, d)
	}

	// Load sessions
	sessRows, err := s.db.QueryContext(ctx, `
		SELECT session_id FROM task_sessions WHERE task_id = ? ORDER BY created_at
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer sessRows.Close()
	for sessRows.Next() {
		var sid string
		if err := sessRows.Scan(&sid); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		task.SessionIDs = append(task.SessionIDs, sid)
	}

	// Load labels
	labelRows, err := s.db.QueryContext(ctx, `
		SELECT label FROM task_labels WHERE task_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get labels: %w", err)
	}
	defer labelRows.Close()
	for labelRows.Next() {
		var l string
		if err := labelRows.Scan(&l); err != nil {
			return nil, fmt.Errorf("failed to scan label: %w", err)
		}
		task.Labels = append(task.Labels, l)
	}

	return task, nil
}

// Update saves changes to an existing task.
func (s *SQLiteTaskStore) Update(ctx context.Context, task *Task) error {
	task.UpdatedAt = time.Now()

	var completedAt interface{}
	if task.CompletedAt != nil {
		completedAt = *task.CompletedAt
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET title=?, objective=?, status=?, priority=?, summary=?, updated_at=?, completed_at=?
		WHERE id=?
	`, task.Title, task.Objective, task.Status, task.Priority, task.Summary, task.UpdatedAt, completedAt, task.ID)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

// Delete removes a task and all related data (cascading).
func (s *SQLiteTaskStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}

// List returns tasks matching the filter.
func (s *SQLiteTaskStore) List(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	var clauses []string
	var args []interface{}

	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Priority != "" {
		clauses = append(clauses, "priority = ?")
		args = append(args, filter.Priority)
	}
	if filter.Label != "" {
		clauses = append(clauses, "id IN (SELECT task_id FROM task_labels WHERE label = ?)")
		args = append(args, filter.Label)
	}

	query := "SELECT id FROM tasks"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan task id: %w", err)
		}
		ids = append(ids, id)
	}

	// Load full task data for each ID
	var tasks []*Task
	for _, id := range ids {
		t, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// AddDirectory adds a bound directory to a task.
func (s *SQLiteTaskStore) AddDirectory(ctx context.Context, taskID string, dir BoundDirectory) error {
	if dir.AddedAt.IsZero() {
		dir.AddedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO task_directories (task_id, path, label, added_at)
		VALUES (?, ?, ?, ?)
	`, taskID, dir.Path, dir.Label, dir.AddedAt)
	if err != nil {
		return fmt.Errorf("failed to add directory: %w", err)
	}
	return nil
}

// RemoveDirectory removes a bound directory from a task.
func (s *SQLiteTaskStore) RemoveDirectory(ctx context.Context, taskID string, path string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM task_directories WHERE task_id = ? AND path = ?
	`, taskID, path)
	if err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}
	return nil
}

// AddSession links a session to a task.
func (s *SQLiteTaskStore) AddSession(ctx context.Context, taskID, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO task_sessions (task_id, session_id) VALUES (?, ?)
	`, taskID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to add session: %w", err)
	}
	return nil
}

// Close closes the database connection if we own it.
func (s *SQLiteTaskStore) Close() error {
	return s.db.Close()
}
