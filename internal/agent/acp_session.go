// Package agent provides coding agent lifecycle management.
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codeg/internal/acp"
)

// AcpSession wraps a single ACP session for a sub-task phase.
// It manages: create → prompt → SSE stream → cancel.
type AcpSession struct {
	client  *acp.ACPClient
	params  SessionParams

	mu       sync.RWMutex
	id       string
	state    acp.SessionState
	events   chan acp.SSEEvent
	cancelFn context.CancelFunc

	// Timestamps
	startedAt   time.Time
	completedAt *time.Time
}

// NewAcpSession creates a new ACP session (does not start it).
func NewAcpSession(client *acp.ACPClient, params SessionParams) (*AcpSession, error) {
	if client == nil {
		return nil, fmt.Errorf("ACP client is nil")
	}
	if params.Cwd == "" {
		return nil, fmt.Errorf("cwd is required for ACP session")
	}
	return &AcpSession{
		client: client,
		params: params,
		state:  acp.SessionInitializing,
		events: make(chan acp.SSEEvent, 128),
	}, nil
}

// Start creates the session on the ACP server, sends the prompt, and begins SSE streaming.
func (s *AcpSession) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != acp.SessionInitializing {
		return fmt.Errorf("cannot start session in state %s", s.state)
	}

	// Create session on server
	sessionID := fmt.Sprintf("codeg-%s-%s-%d", s.params.SubTaskID, s.params.Phase, time.Now().UnixNano())
	sessionResp, err := s.client.CreateSession(ctx, acp.SessionNewParams{
		SessionID:   sessionID,
		WorkspaceID: "codeg",
		Role:        string(s.params.Role),
		Cwd:         s.params.Cwd,
		Name:        s.params.Name,
	})
	if err != nil {
		s.state = acp.SessionError
		return fmt.Errorf("create session: %w", err)
	}

	s.id = sessionResp.SessionID
	s.state = acp.SessionActive
	s.startedAt = time.Now()

	// Send prompt
	_, err = s.client.SendPrompt(ctx, acp.SessionPromptParams{
		SessionID: s.id,
		Prompt:    s.params.Prompt,
	})
	if err != nil {
		s.state = acp.SessionError
		return fmt.Errorf("send prompt: %w", err)
	}

	// Start SSE streaming in background
	streamCtx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel
	s.state = acp.SessionStreaming

	go s.streamEvents(streamCtx)

	return nil
}

// streamEvents subscribes to SSE and forwards events to the channel.
func (s *AcpSession) streamEvents(ctx context.Context) {
	defer close(s.events)

	// Reconnect with backoff for resilience
	ch, err := s.client.Reconnect(ctx, s.id, 5)
	if err != nil {
		s.mu.Lock()
		s.state = acp.SessionError
		s.mu.Unlock()
		s.events <- acp.SSEEvent{
			Type: acp.EventSessionError,
			Data: map[string]interface{}{"message": err.Error()},
		}
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				// Channel closed — session ended
				s.mu.Lock()
				s.state = acp.SessionComplete
				now := time.Now()
				s.completedAt = &now
				s.mu.Unlock()
				return
			}
			s.events <- evt

			// Track state transitions
			if evt.Type == acp.EventTurnComplete {
				s.mu.Lock()
				s.state = acp.SessionComplete
				now := time.Now()
				s.completedAt = &now
				s.mu.Unlock()
			}
		}
	}
}

// Cancel sends a cancel request and stops the SSE stream.
func (s *AcpSession) Cancel() error {
	s.mu.Lock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.id != "" {
		return s.client.CancelSession(ctx, s.id)
	}
	return nil
}

// ID returns the session ID.
func (s *AcpSession) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// State returns the current session state.
func (s *AcpSession) State() acp.SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Events returns the event channel.
func (s *AcpSession) Events() <-chan acp.SSEEvent {
	return s.events
}

// Phase returns the session phase (coding/verifying).
func (s *AcpSession) Phase() string {
	return s.params.Phase
}

// SubTaskID returns the associated sub-task ID.
func (s *AcpSession) SubTaskID() string {
	return s.params.SubTaskID
}

// StartedAt returns when the session started.
func (s *AcpSession) StartedAt() time.Time {
	return s.startedAt
}

// CompletedAt returns when the session completed, or nil.
func (s *AcpSession) CompletedAt() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.completedAt == nil {
		return nil
	}
	t := *s.completedAt
	return &t
}

// IsActive returns true if the session is still running.
func (s *AcpSession) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == acp.SessionActive || s.state == acp.SessionStreaming
}
