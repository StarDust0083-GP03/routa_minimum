// Package acprunner implements AgentRunner using an ACP HTTP+SSE server.
package acprunner

import (
	"context"
	"fmt"
	"time"

	"codeg/internal/acp"
	"codeg/internal/agent"
)

// Runner implements agent.AgentRunner via ACP JSON-RPC + SSE.
type Runner struct {
	client *acp.ACPClient
}

// NewRunner creates a new ACP-based agent runner.
func NewRunner(serverURL string) *Runner {
	return &Runner{
		client: acp.NewACPClient(serverURL),
	}
}

// Start creates an ACP session and subscribes to SSE events.
func (r *Runner) Start(ctx context.Context, cwd, prompt string) (*agent.StartResult, error) {
	sessionID := "session-" + randomID()

	// Create session
	sessionResp, err := r.client.CreateSession(ctx, acp.SessionNewParams{
		SessionID:   sessionID,
		WorkspaceID: "codeg",
		Role:        "DEVELOPER",
		Cwd:         cwd,
	})
	if err != nil {
		return nil, err
	}

	// Send initial prompt
	_, err = r.client.SendPrompt(ctx, acp.SessionPromptParams{
		SessionID: sessionResp.SessionID,
		Prompt:    prompt,
	})
	if err != nil {
		return nil, err
	}

	// Subscribe to SSE events
	events, err := r.client.SubscribeEvents(ctx, sessionResp.SessionID)
	if err != nil {
		return nil, err
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

func randomID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
