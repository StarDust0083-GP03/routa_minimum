package acp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRequest(t *testing.T) {
	req := NewRequest("session/new", SessionNewParams{
		SessionID:   "ses-123",
		WorkspaceID: "codeg",
		Cwd:         "/project",
	})

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %q", req.JSONRPC)
	}
	if req.Method != "session/new" {
		t.Errorf("expected method 'session/new', got %q", req.Method)
	}
	if req.ID <= 0 {
		t.Error("expected ID > 0")
	}
}

func TestRequestIDIncrements(t *testing.T) {
	id1 := NewRequest("m1", nil).ID
	id2 := NewRequest("m2", nil).ID
	if id2 <= id1 {
		t.Errorf("expected IDs to increment: %d -> %d", id1, id2)
	}
}

func TestJSONRPCSerialization(t *testing.T) {
	req := NewRequest("initialize", map[string]interface{}{
		"protocolVersion": 1,
	})

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %v", parsed["jsonrpc"])
	}
	if parsed["method"] != "initialize" {
		t.Errorf("expected method 'initialize', got %v", parsed["method"])
	}
}

func TestSSEParsing(t *testing.T) {
	// Simulate SSE stream
	sseData := `event: agent_message_chunk
data: {"text":"Hello world"}

event: tool_call
data: {"name":"read","args":{"path":"/test"}}

event: turn_complete
data: {}
`

	client := NewACPClient("http://localhost:9999")
	ch := make(chan SSEEvent, 10)

	go func() {
		defer close(ch)
		client.parseSSE(strings.NewReader(sseData), ch)
	}()

	events := make([]SSEEvent, 0, 3)
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Event 1: agent_message_chunk
	if events[0].Type != EventMessageChunk {
		t.Errorf("expected event type %q, got %q", EventMessageChunk, events[0].Type)
	}
	if text, ok := events[0].Data["text"].(string); !ok || text != "Hello world" {
		t.Errorf("expected text 'Hello world', got %v", events[0].Data["text"])
	}

	// Event 2: tool_call
	if events[1].Type != EventToolCall {
		t.Errorf("expected event type %q, got %q", EventToolCall, events[1].Type)
	}

	// Event 3: turn_complete
	if events[2].Type != EventTurnComplete {
		t.Errorf("expected event type %q, got %q", EventTurnComplete, events[2].Type)
	}
}

func TestSSEParsingMultiLineData(t *testing.T) {
	sseData := `event: agent_message_chunk
data: {"text":"Line 1
data: Line 2
data: Line 3"}

`
	client := NewACPClient("http://localhost:9999")
	ch := make(chan SSEEvent, 10)

	go func() {
		defer close(ch)
		client.parseSSE(strings.NewReader(sseData), ch)
	}()

	evt := <-ch
	if evt.Type != EventMessageChunk {
		t.Errorf("expected event type %q, got %q", EventMessageChunk, evt.Type)
	}
}

func TestSSEParsingComments(t *testing.T) {
	sseData := `: this is a comment

event: agent_message_chunk
data: {"text":"after comment"}

`
	client := NewACPClient("http://localhost:9999")
	ch := make(chan SSEEvent, 10)

	go func() {
		defer close(ch)
		client.parseSSE(strings.NewReader(sseData), ch)
	}()

	// Should skip the comment and only get one event
	evt := <-ch
	if evt.Type != EventMessageChunk {
		t.Errorf("expected event type %q, got %q", EventMessageChunk, evt.Type)
	}
}

func TestACPClientHTTPMock(t *testing.T) {
	// Create a mock ACP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodPost {
			// JSON-RPC handler
			var req JSONRPCRequest
			json.NewDecoder(r.Body).Decode(&req)

			var result json.RawMessage
			switch req.Method {
			case "initialize":
				result, _ = json.Marshal(InitializeResult{
					ProtocolVersion: 1,
					ServerName:      "test-server",
				})
			case "session/new":
				result, _ = json.Marshal(SessionResponse{
					SessionID: "ses-test-001",
					Status:    "active",
				})
			case "session/prompt":
				result, _ = json.Marshal(PromptResponse{
					SessionID: "ses-test-001",
					TurnID:    "turn-1",
					Status:    "streaming",
				})
			default:
				result = json.RawMessage(`{"ok":true}`)
			}

			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewACPClient(server.URL)

	// Test Initialize
	initResult, err := client.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if initResult.ProtocolVersion != 1 {
		t.Errorf("expected protocol version 1, got %d", initResult.ProtocolVersion)
	}

	// Test CreateSession
	session, err := client.CreateSession(context.Background(), SessionNewParams{
		SessionID:   "test-ses",
		WorkspaceID: "codeg",
		Cwd:         "/project",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session.SessionID != "ses-test-001" {
		t.Errorf("expected session ID 'ses-test-001', got %q", session.SessionID)
	}

	// Test SendPrompt
	promptResp, err := client.SendPrompt(context.Background(), SessionPromptParams{
		SessionID: "ses-test-001",
		Prompt:    "hello",
	})
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}
	if promptResp.Status != "streaming" {
		t.Errorf("expected status 'streaming', got %q", promptResp.Status)
	}

	// Test Ping
	err = client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}


func TestACPServerConfig_Defaults(t *testing.T) {
	cfg := ACPServerConfig{}

	if cmd := cfg.DefaultCommand(); cmd != "opencode acp" {
		t.Errorf("expected default command 'opencode acp', got %q", cmd)
	}
	if flag := cfg.DefaultPortFlag(); flag != "--port" {
		t.Errorf("expected default port flag '--port', got %q", flag)
	}
}

func TestACPServerConfig_Custom(t *testing.T) {
	cfg := ACPServerConfig{
		Command:  "opencode2 acp",
		PortFlag: "-p",
		Port:     4200,
	}

	if cmd := cfg.DefaultCommand(); cmd != "opencode2 acp" {
		t.Errorf("expected 'opencode2 acp', got %q", cmd)
	}
	if flag := cfg.DefaultPortFlag(); flag != "-p" {
		t.Errorf("expected '-p', got %q", flag)
	}
}

func TestACPServerConfig_CommandArgs_Default(t *testing.T) {
	cfg := ACPServerConfig{}
	binary, args := cfg.CommandArgs()

	if binary != "opencode" {
		t.Errorf("expected binary 'opencode', got %q", binary)
	}
	if len(args) != 1 || args[0] != "acp" {
		t.Errorf("expected args ['acp'], got %v", args)
	}
}

func TestACPServerConfig_CommandArgs_Custom(t *testing.T) {
	cfg := ACPServerConfig{
		Command: "my-acp-server start --verbose",
	}
	binary, args := cfg.CommandArgs()

	if binary != "my-acp-server" {
		t.Errorf("expected binary 'my-acp-server', got %q", binary)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "start" || args[1] != "--verbose" {
		t.Errorf("expected ['start', '--verbose'], got %v", args)
	}
}

func TestSplitCommand_Simple(t *testing.T) {
	parts := splitCommand("opencode serve")
	if len(parts) != 2 || parts[0] != "opencode" || parts[1] != "serve" {
		t.Errorf("expected ['opencode', 'serve'], got %v", parts)
	}
}

func TestSplitCommand_Single(t *testing.T) {
	parts := splitCommand("opencode")
	if len(parts) != 1 || parts[0] != "opencode" {
		t.Errorf("expected ['opencode'], got %v", parts)
	}
}

func TestSplitCommand_WithFlags(t *testing.T) {
	parts := splitCommand("opencode serve --verbose")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d: %v", len(parts), parts)
	}
	if parts[2] != "--verbose" {
		t.Errorf("expected '--verbose', got %q", parts[2])
	}
}

func TestSplitCommand_Empty(t *testing.T) {
	parts := splitCommand("")
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %v", parts)
	}
}

func TestSplitCommand_Spaces(t *testing.T) {
	parts := splitCommand("  opencode   serve  ")
	if len(parts) != 2 || parts[0] != "opencode" || parts[1] != "serve" {
		t.Errorf("expected ['opencode', 'serve'], got %v", parts)
	}
}

func TestACPServerConfig_CommandArgs_EmptyCommand(t *testing.T) {
	cfg := ACPServerConfig{Command: ""}
	binary, args := cfg.CommandArgs()
	if binary != "opencode" || len(args) != 1 || args[0] != "acp" {
		t.Errorf("expected fallback to opencode acp, got %s %v", binary, args)
	}
}
