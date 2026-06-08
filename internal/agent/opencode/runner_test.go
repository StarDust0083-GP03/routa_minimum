package opencode

import (
	"context"
	"os/exec"
	"testing"

	"codeg/internal/acp"
)

func opencodeAvailable() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func TestNewRunner(t *testing.T) {
	runner := NewRunner("")
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.binPath != "opencode" {
		t.Errorf("expected binPath 'opencode', got %q", runner.binPath)
	}
}

func TestParseOpenCodeEvent_Text(t *testing.T) {
	raw := map[string]interface{}{
		"type": "text",
		"part": map[string]interface{}{"text": "Hello world"},
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Type != acp.EventMessageChunk {
		t.Errorf("expected EventMessageChunk, got %q", evt.Type)
	}
	if evt.Data["text"] != "Hello world" {
		t.Errorf("expected 'Hello world', got %v", evt.Data["text"])
	}
}

func TestParseOpenCodeEvent_ToolCall(t *testing.T) {
	raw := map[string]interface{}{
		"type": "tool_call",
		"part": map[string]interface{}{
			"name": "read",
			"args": map[string]interface{}{"path": "/test"},
		},
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Type != acp.EventToolCall {
		t.Errorf("expected EventToolCall, got %q", evt.Type)
	}
}

func TestParseOpenCodeEvent_ToolResult(t *testing.T) {
	raw := map[string]interface{}{
		"type": "tool_result",
		"part": map[string]interface{}{"result": "file contents"},
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Type != acp.EventToolUpdate {
		t.Errorf("expected EventToolUpdate, got %q", evt.Type)
	}
}

func TestParseOpenCodeEvent_StepFinish(t *testing.T) {
	raw := map[string]interface{}{
		"type": "step_finish",
		"part": map[string]interface{}{
			"tokens": map[string]interface{}{"input": 100.0, "output": 50.0},
		},
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Type != acp.EventTurnComplete {
		t.Errorf("expected EventTurnComplete, got %q", evt.Type)
	}
}

func TestParseOpenCodeEvent_Error(t *testing.T) {
	raw := map[string]interface{}{
		"type":    "error",
		"message": "something went wrong",
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Data["message"] != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %v", evt.Data["message"])
	}
}

// Integration test — requires opencode installed
func TestStart_Integration(t *testing.T) {
	if !opencodeAvailable() {
		t.Skip("opencode not found in PATH")
	}

	runner := NewRunner("")
	_, err := runner.Start(context.Background(), "/tmp", "say hello", acp.RoleDeveloper)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
}

func TestParseOpenCodeEvent_ToolUse(t *testing.T) {
	raw := map[string]interface{}{
		"type": "tool_use",
		"part": map[string]interface{}{
			"tool": "bash",
			"state": map[string]interface{}{
				"input":  map[string]interface{}{"command": "ls -la"},
				"output": "file1.txt\nfile2.txt",
			},
		},
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Type != acp.EventToolCall {
		t.Errorf("expected EventToolCall, got %q", evt.Type)
	}
	if evt.Data["name"] != "bash" {
		t.Errorf("expected tool name 'bash', got %v", evt.Data["name"])
	}
}

func TestParseOpenCodeEvent_StepStart(t *testing.T) {
	raw := map[string]interface{}{
		"type": "step_start",
	}
	evt := parseOpenCodeEvent(raw)
	if evt.Type != acp.EventMessageChunk {
		t.Errorf("expected EventMessageChunk, got %q", evt.Type)
	}
}

func TestTruncateStr_Short(t *testing.T) {
	if s := truncateStr("hello", 10); s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}

func TestTruncateStr_Long(t *testing.T) {
	s := truncateStr("0123456789abcdef", 10)
	if len(s) > 13 {
		t.Errorf("too long: %q", s)
	}
}
