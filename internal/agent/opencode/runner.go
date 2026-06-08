// Package opencode implements AgentRunner using the ACP protocol (opencode serve).
package opencode

import (
	"context"
	"fmt"
	"time"

	"codeg/internal/acp"
	"codeg/internal/agent"
)

// Runner implements agent.AgentRunner by communicating with opencode serve via ACP.
type Runner struct {
	client *acp.ACPClient
}

// NewRunner creates a new ACP-based opencode runner.
func NewRunner(serverURL string) *Runner {
	return &Runner{
		client: acp.NewACPClient(serverURL),
	}
}

// Start creates an ACP session via opencode serve, sends a prompt, and streams SSE events.
func (r *Runner) Start(ctx context.Context, cwd, prompt string, role acp.AgentRole) (*agent.StartResult, error) {
	sessionID := fmt.Sprintf("codeg-session-%d", time.Now().UnixNano())

	if role == "" {
		role = acp.RoleDeveloper
	}

	// Initialize ACP handshake (idempotent, non-fatal if unsupported)
	r.client.Initialize(ctx)

	// Create session
	sessionResp, err := r.client.CreateSession(ctx, acp.SessionNewParams{
		SessionID:   sessionID,
		WorkspaceID: "codeg",
		Role:        string(role),
		Cwd:         cwd,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Send initial prompt
	_, err = r.client.SendPrompt(ctx, acp.SessionPromptParams{
		SessionID: sessionResp.SessionID,
		Prompt:    prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	// Subscribe to SSE events
	events, err := r.client.SubscribeEvents(ctx, sessionResp.SessionID)
	if err != nil {
		return nil, fmt.Errorf("subscribe events: %w", err)
	}

	cancel := func() error {
		return r.client.CancelSession(context.Background(), sessionResp.SessionID)
	}

	return &agent.StartResult{
		Events:    events,
		Cancel:    cancel,
		SessionID: sessionResp.SessionID,
	}, nil
}

// Resume reconnects to an existing session and resumes streaming.
func (r *Runner) Resume(ctx context.Context, sessionID string) (*agent.StartResult, error) {
	// Load session to verify it exists
	_, err := r.client.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	// Re-subscribe to SSE events
	events, err := r.client.Reconnect(ctx, sessionID, 3)
	if err != nil {
		return nil, fmt.Errorf("resume events: %w", err)
	}

	cancel := func() error {
		return r.client.CancelSession(context.Background(), sessionID)
	}

	return &agent.StartResult{
		Events:    events,
		Cancel:    cancel,
		SessionID: sessionID,
	}, nil
}

// Load retrieves session history from the ACP server.
func (r *Runner) Load(ctx context.Context, sessionID string) (*acp.SessionResponse, error) {
	return r.client.LoadSession(ctx, sessionID)
}
