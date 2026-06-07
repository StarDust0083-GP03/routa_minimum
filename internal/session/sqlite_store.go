package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SQLiteStore implements SessionStore using SQLite.
type SQLiteStore struct {
	db      *sql.DB
	dbPath  string
	mu      sync.RWMutex
	leafIDs map[string]string // sessionID -> leafEntryID
}

// SQLiteConfig holds configuration for SQLite store.
type SQLiteConfig struct {
	// Path to the SQLite database file.
	Path string

	// EnableWAL enables Write-Ahead Logging for better concurrency.
	EnableWAL bool
}

// NewSQLiteStore creates a new SQLite-based session store.
func NewSQLiteStore(ctx context.Context, cfg SQLiteConfig) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite supports one writer at a time
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{
		db:      db,
		dbPath:  cfg.Path,
		leafIDs: make(map[string]string),
	}

	// Enable WAL mode if requested
	if cfg.EnableWAL {
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
	}

	// Initialize schema
	if err := store.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database schema.
func (s *SQLiteStore) initSchema(ctx context.Context) error {
	schema := `
	-- Sessions table stores session metadata
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		version INTEGER NOT NULL DEFAULT 1,
		cwd TEXT NOT NULL,
		name TEXT,
		parent_session TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Entries table stores all session entries
	CREATE TABLE IF NOT EXISTS entries (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		type TEXT NOT NULL,
		parent_id TEXT,
		timestamp DATETIME NOT NULL,
		data TEXT NOT NULL,  -- JSON-encoded entry data
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		FOREIGN KEY (parent_id) REFERENCES entries(id) ON DELETE CASCADE
	);

	-- Leaf pointers table stores current leaf for each session
	CREATE TABLE IF NOT EXISTS leaf_pointers (
		session_id TEXT PRIMARY KEY,
		entry_id TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
	);

	-- Labels table stores entry labels
	CREATE TABLE IF NOT EXISTS labels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		entry_id TEXT NOT NULL,
		label TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(session_id, entry_id),
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
	);

	-- Indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_entries_session ON entries(session_id);
	CREATE INDEX IF NOT EXISTS idx_entries_parent ON entries(parent_id);
	CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
	CREATE INDEX IF NOT EXISTS idx_sessions_cwd ON sessions(cwd);
	CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Create creates a new session.
func (s *SQLiteStore) Create(ctx context.Context, cwd, name string) (*SessionHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()

	header := &SessionHeader{
		Type:      "session",
		Version:   CurrentSessionVersion,
		ID:        id,
		Timestamp: now,
		Cwd:       cwd,
		Name:      name,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, version, cwd, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, header.Version, cwd, name, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Initialize empty leaf pointer
	s.leafIDs[id] = ""

	return header, nil
}

// Open opens an existing session.
func (s *SQLiteStore) Open(ctx context.Context, sessionID string) (*SessionHeader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	header, err := s.GetHeader(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Load leaf pointer
	var leafID sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT entry_id FROM leaf_pointers WHERE session_id = ?
	`, sessionID).Scan(&leafID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to load leaf pointer: %w", err)
	}

	if leafID.Valid {
		s.leafIDs[sessionID] = leafID.String
	} else {
		s.leafIDs[sessionID] = ""
	}

	return header, nil
}

// Delete deletes a session.
func (s *SQLiteStore) Delete(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	delete(s.leafIDs, sessionID)
	return nil
}

// List lists sessions for a working directory.
func (s *SQLiteStore) List(ctx context.Context, cwd string) ([]*SessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.cwd, s.name, s.parent_session, s.created_at, s.updated_at,
			(SELECT COUNT(*) FROM entries e WHERE e.session_id = s.id AND e.type = 'message') as message_count
		FROM sessions s
		WHERE s.cwd = ?
		ORDER BY s.updated_at DESC
	`, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionInfo
	for rows.Next() {
		info := &SessionInfo{}
		var name, parentSession sql.NullString
		err := rows.Scan(
			&info.ID,
			&info.Cwd,
			&name,
			&parentSession,
			&info.Created,
			&info.Modified,
			&info.MessageCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		if name.Valid {
			info.Name = name.String
		}
		if parentSession.Valid {
			info.ParentSessionPath = parentSession.String
		}
		sessions = append(sessions, info)
	}

	return sessions, rows.Err()
}

// ListAll lists all sessions.
func (s *SQLiteStore) ListAll(ctx context.Context) ([]*SessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.cwd, s.name, s.parent_session, s.created_at, s.updated_at,
			(SELECT COUNT(*) FROM entries e WHERE e.session_id = s.id AND e.type = 'message') as message_count
		FROM sessions s
		ORDER BY s.updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list all sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionInfo
	for rows.Next() {
		info := &SessionInfo{}
		var name, parentSession sql.NullString
		err := rows.Scan(
			&info.ID,
			&info.Cwd,
			&name,
			&parentSession,
			&info.Created,
			&info.Modified,
			&info.MessageCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		if name.Valid {
			info.Name = name.String
		}
		if parentSession.Valid {
			info.ParentSessionPath = parentSession.String
		}
		sessions = append(sessions, info)
	}

	return sessions, rows.Err()
}

// AppendEntry appends an entry to a session.
func (s *SQLiteStore) AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize entry data
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Insert entry
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO entries (id, session_id, type, parent_id, timestamp, data)
		VALUES (?, ?, ?, ?, ?, ?)
	`, entry.ID, sessionID, entry.Type, entry.ParentID, entry.Timestamp, string(data))
	if err != nil {
		return fmt.Errorf("failed to insert entry: %w", err)
	}

	// Update session timestamp
	_, err = s.db.ExecContext(ctx, `
		UPDATE sessions SET updated_at = ? WHERE id = ?
	`, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session timestamp: %w", err)
	}

	// Update leaf pointer
	s.leafIDs[sessionID] = entry.ID
	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO leaf_pointers (session_id, entry_id, updated_at)
		VALUES (?, ?, ?)
	`, sessionID, entry.ID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update leaf pointer: %w", err)
	}

	return nil
}

// GetEntry retrieves an entry by ID.
func (s *SQLiteStore) GetEntry(ctx context.Context, sessionID, entryID string) (*SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	err := s.db.QueryRowContext(ctx, `
		SELECT data FROM entries WHERE id = ? AND session_id = ?
	`, entryID, sessionID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	var entry SessionEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
	}

	return &entry, nil
}

// GetEntries retrieves all entries for a session.
func (s *SQLiteStore) GetEntries(ctx context.Context, sessionID string) ([]*SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT data FROM entries WHERE session_id = ? ORDER BY timestamp
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}
	defer rows.Close()

	var entries []*SessionEntry
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		var entry SessionEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}

// GetChildren retrieves direct children of an entry.
func (s *SQLiteStore) GetChildren(ctx context.Context, sessionID, parentID string) ([]*SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT data FROM entries
		WHERE session_id = ? AND parent_id = ?
		ORDER BY timestamp
	`
	if parentID == "" {
		query = `
			SELECT data FROM entries
			WHERE session_id = ? AND parent_id IS NULL
			ORDER BY timestamp
		`
	}

	var rows *sql.Rows
	var err error
	if parentID == "" {
		rows, err = s.db.QueryContext(ctx, query, sessionID)
	} else {
		rows, err = s.db.QueryContext(ctx, query, sessionID, parentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	var entries []*SessionEntry
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		var entry SessionEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}

// GetBranch retrieves all entries from root to a specific entry.
func (s *SQLiteStore) GetBranch(ctx context.Context, sessionID, leafID string) ([]*SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get all entries
	entries, err := s.GetEntries(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Build parent map
	parentMap := make(map[string]string) // id -> parentId
	entryMap := make(map[string]*SessionEntry)
	for _, e := range entries {
		entryMap[e.ID] = e
		if e.ParentID != nil {
			parentMap[e.ID] = *e.ParentID
		}
	}

	// Walk from leaf to root
	var branch []*SessionEntry
	visited := make(map[string]bool)
	currentID := leafID

	for currentID != "" {
		if visited[currentID] {
			break // Prevent cycles
		}
		visited[currentID] = true

		entry, ok := entryMap[currentID]
		if !ok {
			break
		}
		branch = append([]*SessionEntry{entry}, branch...)

		if pid, ok := parentMap[currentID]; ok {
			currentID = pid
		} else {
			break
		}
	}

	return branch, nil
}

// GetLeafID returns the current leaf entry ID.
func (s *SQLiteStore) GetLeafID(ctx context.Context, sessionID string) (*string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if leafID, ok := s.leafIDs[sessionID]; ok {
		if leafID == "" {
			return nil, nil
		}
		return &leafID, nil
	}

	var leafID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT entry_id FROM leaf_pointers WHERE session_id = ?
	`, sessionID).Scan(&leafID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get leaf ID: %w", err)
	}

	if !leafID.Valid {
		return nil, nil
	}

	return &leafID.String, nil
}

// SetLeafID sets the current leaf entry ID.
func (s *SQLiteStore) SetLeafID(ctx context.Context, sessionID, leafID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.leafIDs[sessionID] = leafID
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO leaf_pointers (session_id, entry_id, updated_at)
		VALUES (?, ?, ?)
	`, sessionID, leafID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set leaf ID: %w", err)
	}

	return nil
}

// GetHeader retrieves the session header.
func (s *SQLiteStore) GetHeader(ctx context.Context, sessionID string) (*SessionHeader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	header := &SessionHeader{}
	var name, parentSession sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, version, cwd, name, parent_session, created_at
		FROM sessions WHERE id = ?
	`, sessionID).Scan(
		&header.ID,
		&header.Version,
		&header.Cwd,
		&name,
		&parentSession,
		&header.Timestamp,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get header: %w", err)
	}

	header.Type = "session"
	if name.Valid {
		header.Name = name.String
	}
	if parentSession.Valid {
		header.ParentSession = parentSession.String
	}

	return header, nil
}

// UpdateHeader updates session header fields.
func (s *SQLiteStore) UpdateHeader(ctx context.Context, sessionID string, updates func(*SessionHeader)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current header
	header, err := s.GetHeader(ctx, sessionID)
	if err != nil {
		return err
	}

	// Apply updates
	updates(header)

	// Save updates
	_, err = s.db.ExecContext(ctx, `
		UPDATE sessions SET name = ?, parent_session = ?, updated_at = ? WHERE id = ?
	`, header.Name, header.ParentSession, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update header: %w", err)
	}

	return nil
}

// Close closes the store.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SetLabel sets a label on an entry.
func (s *SQLiteStore) SetLabel(ctx context.Context, sessionID, entryID, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO labels (session_id, entry_id, label)
		VALUES (?, ?, ?)
	`, sessionID, entryID, label)
	if err != nil {
		return fmt.Errorf("failed to set label: %w", err)
	}

	return nil
}

// GetLabel gets the label for an entry.
func (s *SQLiteStore) GetLabel(ctx context.Context, sessionID, entryID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var label string
	err := s.db.QueryRowContext(ctx, `
		SELECT label FROM labels WHERE session_id = ? AND entry_id = ?
	`, sessionID, entryID).Scan(&label)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get label: %w", err)
	}

	return label, nil
}

// DeleteLabel removes a label from an entry.
func (s *SQLiteStore) DeleteLabel(ctx context.Context, sessionID, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		DELETE FROM labels WHERE session_id = ? AND entry_id = ?
	`, sessionID, entryID)
	if err != nil {
		return fmt.Errorf("failed to delete label: %w", err)
	}

	return nil
}

// GetDB returns the underlying database connection (for advanced use).
func (s *SQLiteStore) GetDB() *sql.DB {
	return s.db
}
