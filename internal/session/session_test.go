package session

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteConfig(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      ":memory:",
		EnableWAL: true,
	}
	if cfg.Path != ":memory:" {
		t.Errorf("expected path ':memory:', got %q", cfg.Path)
	}
	if !cfg.EnableWAL {
		t.Error("expected EnableWAL to be true")
	}
}

func TestNewSQLiteStore(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      ":memory:",
		EnableWAL: true,
	}

	store, err := NewSQLiteStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewSQLiteStore_GetDB(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      ":memory:",
		EnableWAL: false,
	}

	store, err := NewSQLiteStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	db := store.GetDB()
	if db == nil {
		t.Error("expected non-nil DB")
	}

	// Verify DB is alive
	err = db.Ping()
	if err != nil {
		t.Errorf("DB ping failed: %v", err)
	}
}

func TestSessionHeader_Struct(t *testing.T) {
	s := &SessionHeader{
		ID:   "ses-123",
		Name: "test-session",
		Cwd:  "/project/test",
	}

	if s.ID != "ses-123" {
		t.Errorf("expected ID 'ses-123', got %q", s.ID)
	}
	if s.Name != "test-session" {
		t.Errorf("expected name 'test-session', got %q", s.Name)
	}
	if s.Cwd != "/project/test" {
		t.Errorf("expected cwd '/project/test', got %q", s.Cwd)
	}
}

func TestNewSQLiteStore_EmptyPath(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      "",
		EnableWAL: true,
	}
	// SQLite treats empty path as in-memory; this may or may not error
	store, err := NewSQLiteStore(context.Background(), cfg)
	if err != nil {
		t.Logf("NewSQLiteStore with empty path errored (expected): %v", err)
		return
	}
	if store != nil {
		store.Close()
	}
}

func TestNewSQLiteStore_WALEnabled(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      ":memory:",
		EnableWAL: true,
	}

	store, err := NewSQLiteStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	// WAL mode is implicit for in-memory databases;
	// journal_mode reports "memory" which is correct behavior
	var mode string
	err = store.GetDB().QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("failed to query journal mode: %v", err)
	}
	// Both "wal" and "memory" are acceptable for in-memory DBs
	if mode != "wal" && mode != "memory" {
		t.Errorf("unexpected journal_mode: %q", mode)
	}
}

func TestNewSQLiteStore_Close(t *testing.T) {
	cfg := SQLiteConfig{
		Path:      ":memory:",
		EnableWAL: false,
	}

	store, err := NewSQLiteStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
