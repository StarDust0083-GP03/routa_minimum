package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeg/internal/acp"
)

func setupMockACPServer(t *testing.T) (*httptest.Server, *Runner) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle SSE subscription
		if r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "sessionId") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				return
			}
			// Send one event then close
			w.Write([]byte("event: agent_message_chunk\ndata: {\"text\":\"Hello from agent\"}\n\n"))
			flusher.Flush()
			w.Write([]byte("event: turn_complete\ndata: {}\n\n"))
			flusher.Flush()
			return
		}

		// Handle JSON-RPC
		if r.Method == http.MethodPost {
			var req acp.JSONRPCRequest
			json.NewDecoder(r.Body).Decode(&req)

			var result json.RawMessage
			switch req.Method {
			case "initialize":
				result, _ = json.Marshal(map[string]interface{}{
					"protocolVersion": 1,
					"serverName":      "test-opencode",
				})
			case "session/new":
				params := req.Params.(map[string]interface{})
				sid := params["sessionId"].(string)
				result, _ = json.Marshal(acp.SessionResponse{
					SessionID: sid,
					Status:    "active",
				})
			case "session/prompt":
				params := req.Params.(map[string]interface{})
				sid := params["sessionId"].(string)
				result, _ = json.Marshal(acp.PromptResponse{
					SessionID: sid,
					TurnID:    "turn-1",
					Status:    "streaming",
				})
			case "session/cancel":
				result = json.RawMessage(`{"ok":true}`)
			case "session/load":
				result, _ = json.Marshal(acp.SessionResponse{
					SessionID: req.Params.(map[string]interface{})["sessionId"].(string),
					Status:    "active",
				})
			default:
				result = json.RawMessage(`{"ok":true}`)
			}

			resp := acp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))

	runner := NewRunner(server.URL)
	return server, runner
}

func TestNewRunner(t *testing.T) {
	runner := NewRunner("http://localhost:4200/api/acp")
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.client == nil {
		t.Error("expected non-nil client")
	}
}

func TestStart(t *testing.T) {
	server, runner := setupMockACPServer(t)
	defer server.Close()

	result, err := runner.Start(context.Background(), "/project/test", "Write a function")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if result.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	// Read events from the channel
	var events []acp.SSEEvent
	for evt := range result.Events {
		events = append(events, evt)
		if evt.Type == acp.EventTurnComplete {
			break
		}
	}

	if len(events) < 1 {
		t.Error("expected at least 1 event")
	}
}

func TestResume(t *testing.T) {
	server, runner := setupMockACPServer(t)
	defer server.Close()

	result, err := runner.Resume(context.Background(), "existing-session-id")
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if result.SessionID != "existing-session-id" {
		t.Errorf("expected session ID 'existing-session-id', got %q", result.SessionID)
	}
}

func TestLoad(t *testing.T) {
	server, runner := setupMockACPServer(t)
	defer server.Close()

	session, err := runner.Load(context.Background(), "load-session-id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if session.SessionID != "load-session-id" {
		t.Errorf("expected session ID 'load-session-id', got %q", session.SessionID)
	}
}

func TestStart_Cancel(t *testing.T) {
	server, runner := setupMockACPServer(t)
	defer server.Close()

	result, err := runner.Start(context.Background(), "/project", "test prompt")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel should succeed
	err = result.Cancel()
	if err != nil {
		t.Errorf("Cancel failed: %v", err)
	}
}

func TestStart_StreamsEvents(t *testing.T) {
	server, runner := setupMockACPServer(t)
	defer server.Close()

	result, err := runner.Start(context.Background(), "/project", "prompt")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	foundChunk := false
	foundComplete := false
	for evt := range result.Events {
		if evt.Type == acp.EventMessageChunk {
			foundChunk = true
		}
		if evt.Type == acp.EventTurnComplete {
			foundComplete = true
			break
		}
	}

	if !foundChunk {
		t.Error("expected agent_message_chunk event")
	}
	if !foundComplete {
		t.Error("expected turn_complete event")
	}
}
