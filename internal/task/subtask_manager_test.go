package task

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSubTaskStatus_IsTerminal(t *testing.T) {
	if SubTaskPlanned.IsTerminal() {
		t.Error("planned should not be terminal")
	}
	if SubTaskCoding.IsTerminal() {
		t.Error("coding should not be terminal")
	}
	if !SubTaskDone.IsTerminal() {
		t.Error("done should be terminal")
	}
	if !SubTaskFailed.IsTerminal() {
		t.Error("failed should be terminal")
	}
}

func TestSubTaskStatus_IsActive(t *testing.T) {
	if !SubTaskPlanned.IsActive() {
		t.Error("planned should be active")
	}
	if !SubTaskCoding.IsActive() {
		t.Error("coding should be active")
	}
	if SubTaskDone.IsActive() {
		t.Error("done should not be active")
	}
}

func setupManagerTestDB(t *testing.T) (*SubTaskManager, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)

	// Create tasks table and insert test task
	_, err = db.Exec(`
		CREATE TABLE tasks (
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
		INSERT INTO tasks (id, title, objective, status) VALUES ('task-1', 'Test', 'obj', 'pending');
	`)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	store, err := NewSQLiteSubTaskStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	mgr := NewSubTaskManager(store)
	return mgr, db
}

func TestSubTaskManager_CreateFromPlans(t *testing.T) {
	mgr, _ := setupManagerTestDB(t)
	ctx := context.Background()

	plans := []SubTaskPlan{
		{Title: "Auth module", Description: "Add JWT", Directory: "/backend", OrderIndex: 1},
		{Title: "Login page", Description: "Create form", Directory: "/frontend", OrderIndex: 2},
	}

	subTasks, err := mgr.CreateFromPlans(ctx, "task-1", plans)
	if err != nil {
		t.Fatalf("CreateFromPlans failed: %v", err)
	}

	if len(subTasks) != 2 {
		t.Fatalf("expected 2 sub-tasks, got %d", len(subTasks))
	}

	// Verify they were persisted
	list, err := mgr.ListByTask("task-1")
	if err != nil {
		t.Fatalf("ListByTask failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 persisted sub-tasks, got %d", len(list))
	}
}

func TestSubTaskManager_TransitionStatus(t *testing.T) {
	mgr, _ := setupManagerTestDB(t)
	ctx := context.Background()

	plans := []SubTaskPlan{
		{Title: "Test", Description: "desc", Directory: "/dir", OrderIndex: 1},
	}
	subTasks, _ := mgr.CreateFromPlans(ctx, "task-1", plans)
	st := subTasks[0]

	// Transition: planned -> coding
	updated, err := mgr.TransitionStatus(ctx, st.ID, SubTaskCoding)
	if err != nil {
		t.Fatalf("TransitionStatus to coding failed: %v", err)
	}
	if updated.Status != SubTaskCoding {
		t.Errorf("expected status 'coding', got %q", updated.Status)
	}
	if updated.CodingStartedAt == nil {
		t.Error("expected CodingStartedAt to be set")
	}

	// Transition: coding -> verifying
	updated, err = mgr.TransitionStatus(ctx, st.ID, SubTaskVerifying)
	if err != nil {
		t.Fatalf("TransitionStatus to verifying failed: %v", err)
	}
	if updated.Status != SubTaskVerifying {
		t.Errorf("expected status 'verifying', got %q", updated.Status)
	}

	// Transition: verifying -> done
	updated, err = mgr.TransitionStatus(ctx, st.ID, SubTaskDone)
	if err != nil {
		t.Fatalf("TransitionStatus to done failed: %v", err)
	}
	if updated.Status != SubTaskDone {
		t.Errorf("expected status 'done', got %q", updated.Status)
	}
}

func TestSubTaskManager_InvalidTransition(t *testing.T) {
	mgr, _ := setupManagerTestDB(t)
	ctx := context.Background()

	plans := []SubTaskPlan{
		{Title: "Test", Description: "desc", Directory: "/dir", OrderIndex: 1},
	}
	subTasks, _ := mgr.CreateFromPlans(ctx, "task-1", plans)
	st := subTasks[0]

	// Attempt: planned -> done (invalid)
	_, err := mgr.TransitionStatus(ctx, st.ID, SubTaskDone)
	if err == nil {
		t.Error("expected error for planned -> done transition")
	}
}

func TestSubTaskManager_NextPlanned(t *testing.T) {
	mgr, _ := setupManagerTestDB(t)
	ctx := context.Background()

	plans := []SubTaskPlan{
		{Title: "First", Description: "desc", Directory: "/dir", OrderIndex: 1},
		{Title: "Second", Description: "desc", Directory: "/dir", OrderIndex: 2},
	}
	subTasks, _ := mgr.CreateFromPlans(ctx, "task-1", plans)

	// Start first sub-task
	_, _ = mgr.StartCoding(ctx, subTasks[0].ID)

	// Next planned should be second
	next, err := mgr.NextPlanned("task-1")
	if err != nil {
		t.Fatalf("NextPlanned failed: %v", err)
	}
	if next == nil {
		t.Fatal("expected a next sub-task")
	}
	if next.ID != subTasks[1].ID {
		t.Errorf("expected sub-task %q, got %q", subTasks[1].ID, next.ID)
	}
}

func TestSubTaskManager_AllDone(t *testing.T) {
	mgr, _ := setupManagerTestDB(t)
	ctx := context.Background()

	plans := []SubTaskPlan{
		{Title: "Test", Description: "desc", Directory: "/dir", OrderIndex: 1},
	}
	subTasks, _ := mgr.CreateFromPlans(ctx, "task-1", plans)

	// Not done yet
	allDone, err := mgr.AllDone("task-1")
	if err != nil {
		t.Fatalf("AllDone failed: %v", err)
	}
	if allDone {
		t.Error("expected not all done")
	}

	// Mark as done via proper transitions: planned -> coding -> verifying -> done
	_, _ = mgr.StartCoding(ctx, subTasks[0].ID)
	_, _ = mgr.StartVerifying(ctx, subTasks[0].ID)
	_, _ = mgr.MarkDone(ctx, subTasks[0].ID)

	allDone, err = mgr.AllDone("task-1")
	if err != nil {
		t.Fatalf("AllDone failed: %v", err)
	}
	if !allDone {
		t.Error("expected all done")
	}
}

func TestSubTaskManager_CountByStatus(t *testing.T) {
	mgr, _ := setupManagerTestDB(t)
	ctx := context.Background()

	plans := []SubTaskPlan{
		{Title: "A", Description: "desc", Directory: "/dir", OrderIndex: 1},
		{Title: "B", Description: "desc", Directory: "/dir", OrderIndex: 2},
		{Title: "C", Description: "desc", Directory: "/dir", OrderIndex: 3},
	}
	subTasks, _ := mgr.CreateFromPlans(ctx, "task-1", plans)

	// Mark one as coding, one as done (via proper transitions)
	_, _ = mgr.StartCoding(ctx, subTasks[0].ID)
	_, _ = mgr.StartCoding(ctx, subTasks[1].ID)
	_, _ = mgr.StartVerifying(ctx, subTasks[1].ID)
	_, _ = mgr.MarkDone(ctx, subTasks[1].ID)

	counts, err := mgr.CountByStatus("task-1")
	if err != nil {
		t.Fatalf("CountByStatus failed: %v", err)
	}

	if counts[SubTaskPlanned] != 1 {
		t.Errorf("expected 1 planned, got %d", counts[SubTaskPlanned])
	}
	if counts[SubTaskCoding] != 1 {
		t.Errorf("expected 1 coding, got %d", counts[SubTaskCoding])
	}
	if counts[SubTaskDone] != 1 {
		t.Errorf("expected 1 done, got %d", counts[SubTaskDone])
	}
}
