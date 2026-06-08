package core

import (
	"context"
	"strings"
	"testing"
	"time"

	opencodeagent "codeg/internal/agent/opencode"
)

func TestPlanWithACP_Integration(t *testing.T) {
	runner := opencodeagent.NewRunner("")
	ctrl := NewAgentController(runner)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := `You are a technical project planner. Task: Add dark mode toggle to a web app.
Available directories: /tmp.
Decompose into 3 sub-tasks. Return ONLY a JSON array with title/description/directory fields.
Do NOT use any tools — just output the JSON directly.`

	response, err := ctrl.PlanWithACP(ctx, "/tmp", prompt)
	if err != nil {
		t.Fatalf("PlanWithACP failed: %v", err)
	}
	t.Logf("Plan response: %d chars", len(response))

	if len(response) == 0 {
		t.Fatal("expected non-empty response")
	}
	if !strings.Contains(response, "[") {
		t.Errorf("response missing JSON array: %.200s", response)
	}
}
