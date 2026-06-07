package session

import (
	"context"
)

// SessionStore defines the interface for session persistence.
type SessionStore interface {
	// Create creates a new session and returns its ID.
	Create(ctx context.Context, cwd, name string) (*SessionHeader, error)

	// Open opens an existing session by ID.
	Open(ctx context.Context, sessionID string) (*SessionHeader, error)

	// Delete deletes a session by ID.
	Delete(ctx context.Context, sessionID string) error

	// List lists all sessions for a given working directory.
	List(ctx context.Context, cwd string) ([]*SessionInfo, error)

	// ListAll lists all sessions across all directories.
	ListAll(ctx context.Context) ([]*SessionInfo, error)

	// AppendEntry appends an entry to the session.
	AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error

	// GetEntry retrieves an entry by ID.
	GetEntry(ctx context.Context, sessionID, entryID string) (*SessionEntry, error)

	// GetEntries retrieves all entries for a session.
	GetEntries(ctx context.Context, sessionID string) ([]*SessionEntry, error)

	// GetChildren retrieves direct children of an entry.
	GetChildren(ctx context.Context, sessionID, parentID string) ([]*SessionEntry, error)

	// GetBranch retrieves all entries from root to a specific entry.
	GetBranch(ctx context.Context, sessionID, leafID string) ([]*SessionEntry, error)

	// GetLeafID returns the current leaf entry ID.
	GetLeafID(ctx context.Context, sessionID string) (*string, error)

	// SetLeafID sets the current leaf entry ID (for branching).
	SetLeafID(ctx context.Context, sessionID, leafID string) error

	// GetHeader retrieves the session header.
	GetHeader(ctx context.Context, sessionID string) (*SessionHeader, error)

	// UpdateHeader updates session header fields.
	UpdateHeader(ctx context.Context, sessionID string, updates func(*SessionHeader)) error

	// Close closes the store and releases resources.
	Close() error
}

// SessionManager manages session lifecycle and provides high-level operations.
type SessionManager interface {
	// Create creates a new session.
	Create(ctx context.Context, cwd, name string) (*SessionHeader, error)

	// Open opens an existing session by ID.
	Open(ctx context.Context, sessionID string) error

	// ContinueRecent continues the most recent session, or creates a new one.
	ContinueRecent(ctx context.Context, cwd string) (*SessionHeader, error)

	// Fork creates a new session from an existing entry.
	Fork(ctx context.Context, sourceSessionID, sourceEntryID, targetCwd string) (*SessionHeader, error)

	// AppendMessage appends a message entry.
	AppendMessage(ctx context.Context, message AgentMessage) (string, error)

	// AppendThinkingLevelChange appends a thinking level change entry.
	AppendThinkingLevelChange(ctx context.Context, level string) (string, error)

	// AppendModelChange appends a model change entry.
	AppendModelChange(ctx context.Context, provider, modelID string) (string, error)

	// AppendCompaction appends a compaction entry.
	AppendCompaction(ctx context.Context, summary, firstKeptEntryID string, tokensBefore int) (string, error)

	// AppendBranchSummary appends a branch summary entry.
	AppendBranchSummary(ctx context.Context, fromID, summary string) (string, error)

	// AppendCustomEntry appends a custom entry.
	AppendCustomEntry(ctx context.Context, customType string, data any) (string, error)

	// AppendLabel appends or updates a label.
	AppendLabel(ctx context.Context, targetID, label string) (string, error)

	// AppendSessionInfo appends session info.
	AppendSessionInfo(ctx context.Context, name string) (string, error)

	// GetLeafEntry returns the current leaf entry.
	GetLeafEntry() (*SessionEntry, error)

	// GetEntry retrieves an entry by ID.
	GetEntry(entryID string) (*SessionEntry, error)

	// GetChildren returns direct children of an entry.
	GetChildren(parentID string) ([]*SessionEntry, error)

	// GetBranch returns all entries from root to leaf.
	GetBranch() ([]*SessionEntry, error)

	// BuildSessionContext builds the current session context.
	BuildSessionContext() (*SessionContext, error)

	// GetTree returns the session as a tree structure.
	GetTree() ([]*SessionTreeNode, error)

	// Branch creates a new branch from an entry.
	Branch(branchFromID string) error

	// ResetLeaf resets the leaf to the last entry.
	ResetLeaf() error

	// GetHeader returns the session header.
	GetHeader() *SessionHeader

	// GetCwd returns the current working directory.
	GetCwd() string

	// GetSessionID returns the session ID.
	GetSessionID() string

	// IsPersisted returns true if the session is persisted.
	IsPersisted() bool

	// Close closes the session manager.
	Close() error
}

// SessionManagerConfig holds configuration for session manager.
type SessionManagerConfig struct {
	// Store is the backing store for sessions.
	Store SessionStore

	// Cwd is the current working directory.
	Cwd string

	// SessionDir is the directory for session files.
	SessionDir string

	// AutoSave enables automatic saving of entries.
	AutoSave bool
}

// NewEntry creates a new session entry with a generated ID.
func NewEntry(entryType EntryType, parentID *string) *SessionEntry {
	return &SessionEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      entryType,
			ID:        generateID(),
			ParentID:  parentID,
			Timestamp: now(),
		},
	}
}

// generateID generates a unique ID for entries.
func generateID() string {
	return randomString(16)
}
