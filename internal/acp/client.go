// Package acp implements the Agent Communication Protocol (ACP) client.
package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EventHandler is called for each SSE event received from the ACP server.
type EventHandler func(event SSEEvent)

// ACPClient communicates with an ACP-compatible agent server.
type ACPClient struct {
	serverURL string
	rpcClient *http.Client // for JSON-RPC calls (with timeout)
	sseClient *http.Client // for SSE streaming (no timeout)
	onEvent   EventHandler
}

// NewACPClient creates a new ACP client.
func NewACPClient(serverURL string) *ACPClient {
	return &ACPClient{
		serverURL: serverURL,
		rpcClient: &http.Client{Timeout: 10 * time.Second},
		sseClient: &http.Client{Timeout: 0}, // No timeout for long-lived SSE
	}
}

// SetEventHandler sets the callback for incoming SSE events.
func (c *ACPClient) SetEventHandler(handler EventHandler) {
	c.onEvent = handler
}

// CreateSession creates a new ACP session on the server.
func (c *ACPClient) CreateSession(ctx context.Context, params SessionNewParams) (*SessionResponse, error) {
	req := NewRequest("session/new", params)
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}

	var result SessionResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse session response: %w", err)
	}
	return &result, nil
}

// SendPrompt sends a user prompt to an active session.
func (c *ACPClient) SendPrompt(ctx context.Context, params SessionPromptParams) (*PromptResponse, error) {
	req := NewRequest("session/prompt", params)
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send prompt failed: %w", err)
	}

	var result PromptResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompt response: %w", err)
	}
	return &result, nil
}

// CancelSession cancels an active session.
func (c *ACPClient) CancelSession(ctx context.Context, sessionID string) error {
	req := NewRequest("session/cancel", SessionCancelParams{SessionID: sessionID})
	_, err := c.doRPC(ctx, req)
	return err
}

// LoadSession loads session history.
func (c *ACPClient) LoadSession(ctx context.Context, sessionID string) (*SessionResponse, error) {
	req := NewRequest("session/load", SessionLoadParams{SessionID: sessionID})
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	var result SessionResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse load response: %w", err)
	}
	return &result, nil
}

// SubscribeEvents opens an SSE connection and streams events to the channel.
// The returned channel is closed when the connection ends or ctx is cancelled.
func (c *ACPClient) SubscribeEvents(ctx context.Context, sessionID string) (<-chan SSEEvent, error) {
	url := fmt.Sprintf("%s?sessionId=%s", c.serverURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.sseClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SSE connect failed: %w", err)
	}

	ch := make(chan SSEEvent, 64)

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		c.parseSSE(resp.Body, ch)
	}()

	return ch, nil
}

// doRPC performs a single JSON-RPC request.
func (c *ACPClient) doRPC(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.rpcClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp, nil
}

// parseSSE reads Server-Sent Events from the reader and sends them to the channel.
func (c *ACPClient) parseSSE(reader io.Reader, ch chan<- SSEEvent) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	var (
		eventType string
		dataLines []string
	)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line marks end of event
			if len(dataLines) > 0 {
				event := c.buildEvent(eventType, dataLines)
				ch <- event
			}
			eventType = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if strings.HasPrefix(line, ":") {
			// SSE comment, ignore
			continue
		}
	}

	// Handle any remaining event
	if len(dataLines) > 0 {
		event := c.buildEvent(eventType, dataLines)
		ch <- event
	}
}

// buildEvent constructs an SSEEvent from parsed SSE fields.
func (c *ACPClient) buildEvent(eventType string, dataLines []string) SSEEvent {
	event := SSEEvent{
		Type: eventType,
	}

	// Join multi-line data and parse as JSON
	rawData := strings.Join(dataLines, "\n")
	if rawData != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(rawData), &data); err != nil {
			// Non-JSON data; store as text
			data = map[string]interface{}{"text": rawData}
		}
		event.Data = data
	} else {
		event.Data = make(map[string]interface{})
	}

	// Also callback if handler is set (for non-channel usage)
	if c.onEvent != nil {
		c.onEvent(event)
	}

	return event
}

// Ping checks if the ACP server is reachable.
func (c *ACPClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+"/health", nil)
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
