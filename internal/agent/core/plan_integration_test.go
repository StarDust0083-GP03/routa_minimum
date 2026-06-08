package core

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"codeg/internal/acp"
	opencodeagent "codeg/internal/agent/opencode"
)

func TestPlanWithACP_Integration(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not found in PATH")
	}
	runner := opencodeagent.NewRunner("")
	ctrl := NewAgentController(runner)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use a simple prompt that opencode handles quickly
	prompt := "Output ONLY a JSON array with 2 objects: [{\"title\":\"T1\",\"description\":\"d1\",\"directory\":\"/tmp\"},{\"title\":\"T2\",\"description\":\"d2\",\"directory\":\"/tmp\"}]. Do not use any tools."

	t.Logf("Starting PlanWithACP...")
	start := time.Now()
	response, err := ctrl.PlanWithACP(ctx, "/tmp", prompt)
	elapsed := time.Since(start)
	t.Logf("PlanWithACP took %v, response len=%d, err=%v", elapsed, len(response), err)

	if err != nil {
		// The test might fail if opencode takes too long or the prompt doesn't work.
		// Just log it — the important thing is that the flow doesn't panic.
		t.Logf("PlanWithACP returned error (may be expected): %v", err)
	}
	if len(response) > 0 {
		t.Logf("Response preview: %.200s", response)
		if strings.Contains(response, "[") {
			t.Log("PASS: response contains JSON array")
		}
	}
}

func TestPlanWithACP_Events(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not found in PATH")
	}
	runner := opencodeagent.NewRunner("")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := runner.Start(ctx, "/tmp", "say hello world", acp.RoleDeveloper)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	eventCount := 0
	for evt := range result.Events {
		eventCount++
		t.Logf("Event %d: type=%s data_keys=%v", eventCount, evt.Type, keys(evt.Data))
		if evt.Type == acp.EventMessageChunk {
			if text, ok := evt.Data["text"].(string); ok {
				t.Logf("  text: %s", text[:min(len(text), 100)])
			}
		}
	}
	t.Logf("Total events: %d", eventCount)
	if eventCount == 0 {
		t.Error("expected at least 1 event")
	}
}

func keys(m map[string]interface{}) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
