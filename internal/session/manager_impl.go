package session

import (
	"context"
	"fmt"
	"sync"
)

// Manager implements SessionManager interface.
type Manager struct {
	store    SessionStore
	header   *SessionHeader
	cwd      string
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewManager creates a new session manager.
func NewManager(ctx context.Context, cfg SessionManagerConfig) (*Manager, error) {
	childCtx, cancel := context.WithCancel(ctx)

	m := &Manager{
		store:  cfg.Store,
		cwd:    cfg.Cwd,
		ctx:    childCtx,
		cancel: cancel,
	}

	return m, nil
}

// Create creates a new session.
func (m *Manager) Create(ctx context.Context, cwd, name string) (*SessionHeader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	header, err := m.store.Create(ctx, cwd, name)
	if err != nil {
		return nil, err
	}

	m.header = header
	m.cwd = cwd

	return header, nil
}

// Open opens an existing session.
func (m *Manager) Open(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	header, err := m.store.Open(ctx, sessionID)
	if err != nil {
		return err
	}

	m.header = header
	m.cwd = header.Cwd

	return nil
}

// ContinueRecent continues the most recent session or creates a new one.
func (m *Manager) ContinueRecent(ctx context.Context, cwd string) (*SessionHeader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// List sessions for this cwd
	sessions, err := m.store.List(ctx, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	// If there are existing sessions, open the most recent one
	if len(sessions) > 0 {
		header, err := m.store.Open(ctx, sessions[0].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to open recent session: %w", err)
		}
		m.header = header
		m.cwd = cwd
		return header, nil
	}

	// Create a new session
	header, err := m.store.Create(ctx, cwd, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	m.header = header
	m.cwd = cwd

	return header, nil
}

// Fork creates a new session from an existing entry.
func (m *Manager) Fork(ctx context.Context, sourceSessionID, sourceEntryID, targetCwd string) (*SessionHeader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get entries from source session
	branch, err := m.store.GetBranch(ctx, sourceSessionID, sourceEntryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	// Create new session
	header, err := m.store.Create(ctx, targetCwd, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create forked session: %w", err)
	}

	header.ParentSession = sourceSessionID
	if err := m.store.UpdateHeader(ctx, header.ID, func(h *SessionHeader) {
		h.ParentSession = sourceSessionID
	}); err != nil {
		// Cleanup on failure
		_ = m.store.Delete(ctx, header.ID)
		return nil, fmt.Errorf("failed to update parent session: %w", err)
	}

	// Copy entries to new session
	for _, entry := range branch {
		entryCopy := *entry
		entryCopy.ID = generateID()
		if err := m.store.AppendEntry(ctx, header.ID, &entryCopy); err != nil {
			_ = m.store.Delete(ctx, header.ID)
			return nil, fmt.Errorf("failed to copy entry: %w", err)
		}
	}

	m.header = header
	m.cwd = targetCwd

	return header, nil
}

// AppendMessage appends a message entry.
func (m *Manager) AppendMessage(ctx context.Context, message AgentMessage) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeMessage, leafID)
	entry.Message = &message

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendThinkingLevelChange appends a thinking level change.
func (m *Manager) AppendThinkingLevelChange(ctx context.Context, level string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeThinkingLevelChange, leafID)
	entry.ThinkingLevel = &level

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendModelChange appends a model change.
func (m *Manager) AppendModelChange(ctx context.Context, provider, modelID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeModelChange, leafID)
	entry.Provider = &provider
	entry.ModelID = &modelID

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendCompaction appends a compaction entry.
func (m *Manager) AppendCompaction(ctx context.Context, summary, firstKeptEntryID string, tokensBefore int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeCompaction, leafID)
	entry.Summary = &summary
	entry.FirstKeptEntryID = &firstKeptEntryID
	entry.TokensBefore = &tokensBefore

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendBranchSummary appends a branch summary.
func (m *Manager) AppendBranchSummary(ctx context.Context, fromID, summary string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeBranchSummary, leafID)
	entry.FromID = &fromID
	entry.Summary = &summary

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendCustomEntry appends a custom entry.
func (m *Manager) AppendCustomEntry(ctx context.Context, customType string, data any) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeCustom, leafID)
	entry.CustomType = &customType
	entry.Data = data

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendLabel appends a label.
func (m *Manager) AppendLabel(ctx context.Context, targetID, label string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeLabel, leafID)
	entry.TargetID = &targetID
	entry.Label = &label

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// AppendSessionInfo appends session info.
func (m *Manager) AppendSessionInfo(ctx context.Context, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return "", fmt.Errorf("no active session")
	}

	leafID := m.getLeafID()
	entry := NewEntry(EntryTypeSessionInfo, leafID)
	entry.Name = &name

	if err := m.store.AppendEntry(ctx, m.header.ID, entry); err != nil {
		return "", err
	}

	return entry.ID, nil
}

// GetLeafEntry returns the current leaf entry.
func (m *Manager) GetLeafEntry() (*SessionEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.header == nil {
		return nil, fmt.Errorf("no active session")
	}

	leafID, err := m.store.GetLeafID(m.ctx, m.header.ID)
	if err != nil {
		return nil, err
	}
	if leafID == nil {
		return nil, nil
	}

	return m.store.GetEntry(m.ctx, m.header.ID, *leafID)
}

// GetEntry retrieves an entry by ID.
func (m *Manager) GetEntry(entryID string) (*SessionEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.header == nil {
		return nil, fmt.Errorf("no active session")
	}

	return m.store.GetEntry(m.ctx, m.header.ID, entryID)
}

// GetChildren returns direct children of an entry.
func (m *Manager) GetChildren(parentID string) ([]*SessionEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.header == nil {
		return nil, fmt.Errorf("no active session")
	}

	return m.store.GetChildren(m.ctx, m.header.ID, parentID)
}

// GetBranch returns all entries from root to leaf.
func (m *Manager) GetBranch() ([]*SessionEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.header == nil {
		return nil, fmt.Errorf("no active session")
	}

	leafID, err := m.store.GetLeafID(m.ctx, m.header.ID)
	if err != nil {
		return nil, err
	}
	if leafID == nil {
		return []*SessionEntry{}, nil
	}

	return m.store.GetBranch(m.ctx, m.header.ID, *leafID)
}

// BuildSessionContext builds the current session context.
func (m *Manager) BuildSessionContext() (*SessionContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.header == nil {
		return nil, fmt.Errorf("no active session")
	}

	branch, err := m.store.GetBranch(m.ctx, m.header.ID, "")
	if err != nil {
		return nil, err
	}

	leafID, err := m.store.GetLeafID(m.ctx, m.header.ID)
	if err != nil {
		return nil, err
	}

	var leafIDStr string
	if leafID != nil {
		leafIDStr = *leafID
	}

	branch, err = m.store.GetBranch(m.ctx, m.header.ID, leafIDStr)
	if err != nil {
		return nil, err
	}

	ctx := &SessionContext{
		Messages:      []AgentMessage{},
		ThinkingLevel: "default",
	}

	for _, entry := range branch {
		switch entry.Type {
		case EntryTypeMessage:
			if entry.Message != nil {
				ctx.Messages = append(ctx.Messages, *entry.Message)
			}
		case EntryTypeThinkingLevelChange:
			if entry.ThinkingLevel != nil {
				ctx.ThinkingLevel = *entry.ThinkingLevel
			}
		case EntryTypeModelChange:
			if entry.Provider != nil && entry.ModelID != nil {
				ctx.Model = &ModelInfo{
					Provider: *entry.Provider,
					ModelID:  *entry.ModelID,
				}
			}
		}
	}

	return ctx, nil
}

// GetTree returns the session as a tree structure.
func (m *Manager) GetTree() ([]*SessionTreeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.header == nil {
		return nil, fmt.Errorf("no active session")
	}

	entries, err := m.store.GetEntries(m.ctx, m.header.ID)
	if err != nil {
		return nil, err
	}

	// Build entry map and children map
	entryMap := make(map[string]*SessionEntry)
	childrenMap := make(map[string][]*SessionEntry)
	var roots []*SessionEntry

	for _, e := range entries {
		entryMap[e.ID] = e
		if e.ParentID == nil || *e.ParentID == "" {
			roots = append(roots, e)
		} else {
			childrenMap[*e.ParentID] = append(childrenMap[*e.ParentID], e)
		}
	}

	// Build tree recursively
	var buildTree func(entry *SessionEntry) *SessionTreeNode
	buildTree = func(entry *SessionEntry) *SessionTreeNode {
		node := &SessionTreeNode{
			Entry:    *entry,
			Children: []*SessionTreeNode{},
		}

		for _, child := range childrenMap[entry.ID] {
			node.Children = append(node.Children, buildTree(child))
		}

		return node
	}

	var tree []*SessionTreeNode
	for _, root := range roots {
		tree = append(tree, buildTree(root))
	}

	return tree, nil
}

// Branch creates a new branch from an entry.
func (m *Manager) Branch(branchFromID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return fmt.Errorf("no active session")
	}

	// Verify the entry exists
	entry, err := m.store.GetEntry(m.ctx, m.header.ID, branchFromID)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("entry not found: %s", branchFromID)
	}

	return m.store.SetLeafID(m.ctx, m.header.ID, branchFromID)
}

// ResetLeaf resets the leaf to the last entry.
func (m *Manager) ResetLeaf() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.header == nil {
		return fmt.Errorf("no active session")
	}

	entries, err := m.store.GetEntries(m.ctx, m.header.ID)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return m.store.SetLeafID(m.ctx, m.header.ID, "")
	}

	// Find the last entry (most recent timestamp)
	lastEntry := entries[0]
	for _, e := range entries[1:] {
		if e.Timestamp.After(lastEntry.Timestamp) {
			lastEntry = e
		}
	}

	return m.store.SetLeafID(m.ctx, m.header.ID, lastEntry.ID)
}

// GetHeader returns the session header.
func (m *Manager) GetHeader() *SessionHeader {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.header
}

// GetCwd returns the current working directory.
func (m *Manager) GetCwd() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cwd
}

// GetSessionID returns the session ID.
func (m *Manager) GetSessionID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.header == nil {
		return ""
	}
	return m.header.ID
}

// IsPersisted returns true if the session is persisted.
func (m *Manager) IsPersisted() bool {
	return m.store != nil
}

// Close closes the session manager.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cancel()

	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

// getLeafID returns the current leaf ID for the active session.
func (m *Manager) getLeafID() *string {
	leafID, err := m.store.GetLeafID(m.ctx, m.header.ID)
	if err != nil || leafID == nil {
		return nil
	}
	return leafID
}
