// Package agent provides coding agent lifecycle management.
package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeg/internal/acp"
)

// AcpManager manages the opencode serve child process and ACP session registry.
type AcpManager struct {
	mu sync.RWMutex

	// Server lifecycle
	serverCmd *exec.Cmd
	serverURL string
	cfg       acp.ACPServerConfig

	// Session registry
	sessions map[string]*AcpSession

	// Client for communication
	client *acp.ACPClient

	// Event dispatch
	eventCh chan SessionEvent
}

// SessionEvent represents a lifecycle event for a session.
type SessionEvent struct {
	SessionID string
	SubTaskID string
	Phase     string
	Event     acp.SSEEvent
}

// NewAcpManager creates a new ACP manager with the given server config.
func NewAcpManager(cfg acp.ACPServerConfig) *AcpManager {
	return &AcpManager{
		cfg:      cfg,
		sessions: make(map[string]*AcpSession),
		eventCh:  make(chan SessionEvent, 256),
	}
}

// StartServer launches opencode serve as a child process.
// If cfg.Port is 0, a random port is used and parsed from server output.
func (m *AcpManager) StartServer(ctx context.Context) error {
	args := []string{"serve"}
	if m.cfg.Port > 0 {
		args = append(args, "--port", strconv.Itoa(m.cfg.Port))
	} else {
		args = append(args, "--port", "0") // random port
	}
	if m.cfg.Provider != "" {
		args = append(args, "--provider", m.cfg.Provider)
	}
	if m.cfg.Model != "" {
		args = append(args, "--model", m.cfg.Model)
	}

	cmd := exec.CommandContext(ctx, "opencode", args...)

	// Capture stdout to parse the port
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start opencode serve: %w", err)
	}

	// Parse the port from server output (format: "ACP server listening on :<port>")
	portCh := make(chan int, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if port, err := parsePortFromLine(line); err == nil {
				portCh <- port
				return
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if port, err := parsePortFromLine(line); err == nil {
				portCh <- port
				return
			}
		}
	}()

	// Wait for port with timeout
	select {
	case port := <-portCh:
		m.serverURL = fmt.Sprintf("http://localhost:%d/api/acp", port)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for server port")
	}

	m.serverCmd = cmd
	m.client = acp.NewACPClient(m.serverURL)

	// Wait for server to be healthy
	if err := m.client.HealthCheck(ctx, 10, 200*time.Millisecond); err != nil {
		return fmt.Errorf("server health check failed: %w", err)
	}

	// Initialize ACP handshake (idempotent, may be unsupported by some servers)
	m.client.Initialize(ctx)

	return nil
}

// StopServer gracefully stops the opencode serve process.
func (m *AcpManager) StopServer() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel all active sessions
	for id, session := range m.sessions {
		_ = session.Cancel()
		delete(m.sessions, id)
	}

	if m.serverCmd == nil || m.serverCmd.Process == nil {
		return nil
	}

	if err := m.serverCmd.Process.Signal(os.Interrupt); err != nil {
		// Force kill if graceful shutdown fails
		_ = m.serverCmd.Process.Kill()
	}
	return nil
}

// CreateSession creates a new ACP session for a sub-task phase.
func (m *AcpManager) CreateSession(ctx context.Context, params SessionParams) (*AcpSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil, fmt.Errorf("ACP server not started")
	}

	session, err := NewAcpSession(m.client, params)
	if err != nil {
		return nil, err
	}

	if err := session.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}

	m.sessions[session.ID()] = session

	// Start forwarding events
	go m.forwardEvents(session)

	return session, nil
}

// GetSession returns a session by ID.
func (m *AcpManager) GetSession(sessionID string) (*AcpSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

// RemoveSession removes a session from the registry.
func (m *AcpManager) RemoveSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// SessionCount returns the number of active sessions.
func (m *AcpManager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Events returns the channel for session lifecycle events.
func (m *AcpManager) Events() <-chan SessionEvent {
	return m.eventCh
}

// ServerURL returns the ACP server URL.
func (m *AcpManager) ServerURL() string {
	return m.serverURL
}

// Client returns the ACP client for direct use.
func (m *AcpManager) Client() *acp.ACPClient {
	return m.client
}

// forwardEvents reads events from a session and dispatches them.
func (m *AcpManager) forwardEvents(session *AcpSession) {
	for evt := range session.Events() {
		m.eventCh <- SessionEvent{
			SessionID: session.ID(),
			SubTaskID: session.params.SubTaskID,
			Phase:     session.params.Phase,
			Event:     evt,
		}
	}
}

// parsePortFromLine tries to extract a port number from a log line.
func parsePortFromLine(line string) (int, error) {
	// Look for patterns like "listening on :4200" or "port 4200"
	lower := strings.ToLower(line)
	idx := strings.LastIndex(lower, ":")
	if idx < 0 {
		return 0, fmt.Errorf("no port found in: %s", line)
	}
	portStr := strings.TrimSpace(lower[idx+1:])
	// Extract just the numeric part
	portStr = strings.TrimRight(portStr, " \t\r\n")
	portStr = strings.Split(portStr, " ")[0]
	return strconv.Atoi(portStr)
}

// SessionParams holds parameters for creating an ACP session.
type SessionParams struct {
	SubTaskID string
	Phase     string // "coding" or "verifying"
	Role      acp.AgentRole
	Cwd       string
	Prompt    string
	Name      string
}

