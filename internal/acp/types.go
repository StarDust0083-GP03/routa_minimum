// Package acp implements the Agent Communication Protocol (ACP) client.
// ACP uses JSON-RPC 2.0 over HTTP with SSE for streaming agent updates.
package acp

import (
	"context"
	"encoding/json"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SessionNewParams configures a new ACP session.
type SessionNewParams struct {
	SessionID   string `json:"sessionId"`
	WorkspaceID string `json:"workspaceId"`
	Provider    string `json:"provider,omitempty"`
	Role        string `json:"role,omitempty"`
	Cwd         string `json:"cwd,omitempty"`
	Name        string `json:"name,omitempty"`
}

// SessionPromptParams sends a prompt to an active session.
type SessionPromptParams struct {
	SessionID       string  `json:"sessionId"`
	Prompt          string  `json:"prompt"`
	ParentSessionID *string `json:"parentSessionId,omitempty"`
}

// SessionCancelParams cancels an active session.
type SessionCancelParams struct {
	SessionID string `json:"sessionId"`
}

// SessionLoadParams loads session history.
type SessionLoadParams struct {
	SessionID string `json:"sessionId"`
}

// SessionResponse is the result of a session/new call.
type SessionResponse struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

// PromptResponse is the result of a session/prompt call.
type PromptResponse struct {
	SessionID string `json:"sessionId"`
	TurnID    string `json:"turnId"`
	Status    string `json:"status"`
}

// SessionState tracks the lifecycle state of an ACP session.
type SessionState string

const (
	SessionInitializing SessionState = "initializing"
	SessionActive       SessionState = "active"
	SessionStreaming    SessionState = "streaming"
	SessionComplete     SessionState = "complete"
	SessionCancelled    SessionState = "cancelled"
	SessionError        SessionState = "error"
)

// ACPServerConfig holds configuration for the ACP server process.
type ACPServerConfig struct {
	Port     int    `json:"port"`     // 0 = random port
	Provider string `json:"provider"` // e.g. "anthropic", "openai"
	Model    string `json:"model"`    // model override
}

// SSESubscription wraps an active SSE connection.
type SSESubscription struct {
	SessionID string
	Events    <-chan SSEEvent
	cancel    context.CancelFunc
}

// SSE event type constants.
const (
	EventMessageChunk    = "agent_message_chunk"
	EventToolCall        = "tool_call"
	EventToolUpdate      = "tool_call_update"
	EventProcessOutput   = "process_output"
	EventTurnComplete    = "turn_complete"
	EventUsageUpdate     = "usage_update"
	EventSessionError    = "error"
)

// SSEEvent represents a parsed Server-Sent Event.
type SSEEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// NewRequest creates a JSON-RPC 2.0 request with an auto-incrementing ID.
func NewRequest(method string, params interface{}) JSONRPCRequest {
	requestID := nextID()
	return JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  method,
		Params:  params,
	}
}

var idCounter int

func nextID() int {
	idCounter++
	return idCounter
}

// AgentRole defines the role an agent plays in a session.
type AgentRole string

const (
	RoleDeveloper AgentRole = "DEVELOPER"
	RoleGate      AgentRole = "GATE"
)
