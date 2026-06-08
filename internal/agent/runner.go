// Package agent provides coding agent lifecycle management.
package agent

import (
	"context"

	"codeg/internal/acp"
)

// StartResult holds the results of starting an agent.
type StartResult struct {
	Events    <-chan acp.SSEEvent
	Cancel    func() error
	SessionID string // the agent's session ID, if available
}

// AgentRunner is the interface for running coding agents.
type AgentRunner interface {
	// Start launches a coding agent in the given directory with a prompt.
	Start(ctx context.Context, cwd, prompt string) (*StartResult, error)

	// Resume reconnects to an existing session and continues streaming.
	Resume(ctx context.Context, sessionID string) (*StartResult, error)

	// Load retrieves the history of a past session.
	Load(ctx context.Context, sessionID string) (*acp.SessionResponse, error)
}
