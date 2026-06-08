package task

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)

	// Enable foreign keys (required for cascade delete in SQLite)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Create tasks table (needed for FK)
	_, err = db.Exec(`
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
		)
	`)
	if err != nil {
		t.Fatalf("failed to create tasks table: %v", err)
	}

	// Insert a test task
	_, err = db.Exec(`INSERT INTO tasks (id, title, objective, status) VALUES ('task-1', 'Test Task', 'Test objective', 'pending')`)
	if err != nil {
		t.Fatalf("failed to insert test task: %v", err)
	}

	return db
}

func TestSubTaskStore_Create(t *testing.T) {
	db := setupTestDB(t)
	store, err := NewSQLiteSubTaskStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	st := &SubTask{
		TaskID:      "task-1",
		Title:       "Implement auth",
		Description: "Add JWT authentication middleware",
		Directory:   "/project/backend",
		Status:      SubTaskPlanned,
		OrderIndex:  1,
	}

	err = store.Create(context.Background(), st)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if st.ID == "" {
		t.Error("expected ID to be set")
	}
	if st.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestSubTaskStore_Get(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	st := &SubTask{
		TaskID:      "task-1",
		Title:       "Implement auth",
		Description: "Add JWT authentication middleware",
		Directory:   "/project/backend",
		Status:      SubTaskPlanned,
		OrderIndex:  1,
	}
	_ = store.Create(context.Background(), st)

	got, err := store.Get(context.Background(), st.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Title != st.Title {
		t.Errorf("expected title %q, got %q", st.Title, got.Title)
	}
	if got.TaskID != "task-1" {
		t.Errorf("expected taskID 'task-1', got %q", got.TaskID)
	}
	if got.Status != SubTaskPlanned {
		t.Errorf("expected status 'planned', got %q", got.Status)
	}
}

func TestSubTaskStore_GetNotFound(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	_, err := store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent sub-task")
	}
}

func TestSubTaskStore_ListByTask(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	// Create two sub-tasks
	for i, title := range []string{"First", "Second"} {
		st := &SubTask{
			TaskID:      "task-1",
			Title:       title,
			Description: "desc",
			Directory:   "/project",
			Status:      SubTaskPlanned,
			OrderIndex:  i + 1,
		}
		_ = store.Create(context.Background(), st)
	}

	results, err := store.ListByTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("ListByTask failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 sub-tasks, got %d", len(results))
	}

	// Verify ordering
	if results[0].Title != "First" {
		t.Errorf("expected first sub-task, got %q", results[0].Title)
	}
	if results[1].Title != "Second" {
		t.Errorf("expected second sub-task, got %q", results[1].Title)
	}
}

func TestSubTaskStore_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	st := &SubTask{
		TaskID:     "task-1",
		Title:      "Test",
		Directory:  "/project",
		OrderIndex: 1,
	}
	_ = store.Create(context.Background(), st)

	err := store.UpdateStatus(context.Background(), st.ID, SubTaskCoding)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := store.Get(context.Background(), st.ID)
	if got.Status != SubTaskCoding {
		t.Errorf("expected status 'coding', got %q", got.Status)
	}
}

func TestSubTaskStore_Update(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	st := &SubTask{
		TaskID:     "task-1",
		Title:      "Test",
		Directory:  "/project",
		OrderIndex: 1,
	}
	_ = store.Create(context.Background(), st)

	st.Title = "Updated title"
	st.Description = "Updated desc"
	err := store.Update(context.Background(), st)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := store.Get(context.Background(), st.ID)
	if got.Title != "Updated title" {
		t.Errorf("expected title 'Updated title', got %q", got.Title)
	}
}

func TestSubTaskStore_SetSessionID(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	st := &SubTask{
		TaskID:     "task-1",
		Title:      "Test",
		Directory:  "/project",
		OrderIndex: 1,
	}
	_ = store.Create(context.Background(), st)

	err := store.SetSessionID(context.Background(), st.ID, PhaseCoding, "ses_abc123")
	if err != nil {
		t.Fatalf("SetSessionID failed: %v", err)
	}

	got, _ := store.Get(context.Background(), st.ID)
	if got.CodingSessionID != "ses_abc123" {
		t.Errorf("expected coding session ID 'ses_abc123', got %q", got.CodingSessionID)
	}
}

func TestSubTaskStore_SetPhaseTimestamp(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	st := &SubTask{
		TaskID:     "task-1",
		Title:      "Test",
		Directory:  "/project",
		OrderIndex: 1,
	}
	_ = store.Create(context.Background(), st)

	// Set coding_started
	err := store.SetPhaseTimestamp(context.Background(), st.ID, "coding_started", st.CreatedAt)
	if err != nil {
		t.Fatalf("SetPhaseTimestamp failed: %v", err)
	}

	got, _ := store.Get(context.Background(), st.ID)
	if got.CodingStartedAt == nil {
		t.Error("expected CodingStartedAt to be set")
	}
}

func TestSubTaskStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	st := &SubTask{
		TaskID:     "task-1",
		Title:      "Test",
		Directory:  "/project",
		OrderIndex: 1,
	}
	_ = store.Create(context.Background(), st)

	err := store.Delete(context.Background(), st.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(context.Background(), st.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestSubTaskStore_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	store, _ := NewSQLiteSubTaskStore(db)

	// Create sub-tasks
	for i := 0; i < 3; i++ {
		st := &SubTask{
			TaskID:     "task-1",
			Title:      "Test",
			Directory:  "/project",
			OrderIndex: i,
		}
		_ = store.Create(context.Background(), st)
	}

	// Delete the parent task
	_, err := db.Exec(`DELETE FROM tasks WHERE id = 'task-1'`)
	if err != nil {
		t.Fatalf("failed to delete task: %v", err)
	}

	// Verify sub-tasks are cascade-deleted
	results, _ := store.ListByTask(context.Background(), "task-1")
	if len(results) != 0 {
		t.Errorf("expected 0 sub-tasks after cascade delete, got %d", len(results))
	}
}

func skipIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 to run integration tests")
	}
}
