// Package session provides session management for coding agent.
// It supports tree-structured, append-only session entries with branching capability.
package session

import (
	"time"
)

// CurrentSessionVersion is the current version of session format.
const CurrentSessionVersion = 1

// SessionHeader represents the metadata header of a session.
type SessionHeader struct {
	Type          string     `json:"type"`
	Version       int        `json:"version"`
	ID            string     `json:"id"`
	Timestamp     time.Time  `json:"timestamp"`
	Cwd           string     `json:"cwd"`
	ParentSession string     `json:"parentSession,omitempty"`
	Name          string     `json:"name,omitempty"`
}

// EntryType defines the type of session entry.
type EntryType string

const (
	EntryTypeMessage            EntryType = "message"
	EntryTypeThinkingLevelChange EntryType = "thinking_level_change"
	EntryTypeModelChange        EntryType = "model_change"
	EntryTypeCompaction         EntryType = "compaction"
	EntryTypeBranchSummary      EntryType = "branch_summary"
	EntryTypeCustom             EntryType = "custom"
	EntryTypeLabel              EntryType = "label"
	EntryTypeSessionInfo        EntryType = "session_info"
)

// SessionEntryBase is the base interface for all session entries.
type SessionEntryBase struct {
	Type      EntryType `json:"type"`
	ID        string    `json:"id"`
	ParentID  *string   `json:"parentId"`
	Timestamp time.Time `json:"timestamp"`
}

// MessageRole defines the role of a message sender.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
	RoleCustom    MessageRole = "custom"
)

// MessageContent represents a content block in a message.
type MessageContent struct {
	Type     string `json:"type"` // "text", "image", "tool_use", "tool_result"
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	ToolID   string `json:"toolId,omitempty"`
	ToolName string `json:"toolName,omitempty"`
	Result   string `json:"result,omitempty"`
}

// AgentMessage represents a message in the conversation.
type AgentMessage struct {
	Role    MessageRole      `json:"role"`
	Content []MessageContent `json:"content"`
}

// SessionMessageEntry represents a message entry in the session.
type SessionMessageEntry struct {
	SessionEntryBase
	Message AgentMessage `json:"message"`
}

// ThinkingLevelChangeEntry records a thinking level change.
type ThinkingLevelChangeEntry struct {
	SessionEntryBase
	ThinkingLevel string `json:"thinkingLevel"`
}

// ModelChangeEntry records a model change.
type ModelChangeEntry struct {
	SessionEntryBase
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// CompactionEntry records a context compaction event.
type CompactionEntry struct {
	SessionEntryBase
	Summary         string `json:"summary"`
	FirstKeptEntryID string `json:"firstKeptEntryId"`
	TokensBefore    int    `json:"tokensBefore"`
	FromHook        bool   `json:"fromHook,omitempty"`
}

// BranchSummaryEntry records a session branch summary.
type BranchSummaryEntry struct {
	SessionEntryBase
	FromID   string `json:"fromId"`
	Summary  string `json:"summary"`
	FromHook bool   `json:"fromHook,omitempty"`
}

// CustomEntry allows extensions to store custom data.
type CustomEntry struct {
	SessionEntryBase
	CustomType string `json:"customType"`
	Data       any    `json:"data,omitempty"`
}

// LabelEntry represents a label/bookmark on an entry.
type LabelEntry struct {
	SessionEntryBase
	TargetID string  `json:"targetId"`
	Label    *string `json:"label"`
}

// SessionInfoEntry records session metadata changes.
type SessionInfoEntry struct {
	SessionEntryBase
	Name string `json:"name,omitempty"`
}

// SessionEntry is a union type for all entry types.
type SessionEntry struct {
	SessionEntryBase
	Message            *AgentMessage         `json:"message,omitempty"`
	ThinkingLevel      *string               `json:"thinkingLevel,omitempty"`
	Provider           *string               `json:"provider,omitempty"`
	ModelID            *string               `json:"modelId,omitempty"`
	Summary            *string               `json:"summary,omitempty"`
	FirstKeptEntryID   *string               `json:"firstKeptEntryId,omitempty"`
	TokensBefore       *int                  `json:"tokensBefore,omitempty"`
	FromHook           *bool                 `json:"fromHook,omitempty"`
	FromID             *string               `json:"fromId,omitempty"`
	CustomType         *string               `json:"customType,omitempty"`
	Data               any                   `json:"data,omitempty"`
	TargetID           *string               `json:"targetId,omitempty"`
	Label              *string               `json:"label,omitempty"`
	Name               *string               `json:"name,omitempty"`
}

// SessionContext holds the current context of a session.
type SessionContext struct {
	Messages      []AgentMessage
	ThinkingLevel string
	Model         *ModelInfo
}

// ModelInfo holds information about the current model.
type ModelInfo struct {
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// SessionInfo provides summary information about a session.
type SessionInfo struct {
	Path             string
	ID               string
	Cwd              string
	Name             string
	ParentSessionPath string
	Created          time.Time
	Modified         time.Time
	MessageCount     int
	FirstMessage     string
}

// SessionTreeNode represents a node in the session tree.
type SessionTreeNode struct {
	Entry    SessionEntry        `json:"entry"`
	Children []*SessionTreeNode  `json:"children"`
	Label    string              `json:"label,omitempty"`
}

// IsMessage checks if the entry is a message entry.
func (e *SessionEntry) IsMessage() bool {
	return e.Type == EntryTypeMessage
}

// IsCompaction checks if the entry is a compaction entry.
func (e *SessionEntry) IsCompaction() bool {
	return e.Type == EntryTypeCompaction
}

// GetParentID returns the parent ID or empty string if nil.
func (e *SessionEntry) GetParentID() string {
	if e.ParentID == nil {
		return ""
	}
	return *e.ParentID
}
