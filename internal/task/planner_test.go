package task

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeg/internal/llm"
)

// mockLLMClient implements llm.Client for testing.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Chat(ctx context.Context, systemPrompt string, messages []llm.Message) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockLLMClient) Summarize(ctx context.Context, objective, conversation string) (string, error) {
	return "summary", nil
}

func TestParsePlans_ValidJSON(t *testing.T) {
	response := `[
		{"title": "Auth middleware", "description": "Add JWT auth", "directory": "/backend"},
		{"title": "Login page", "description": "Create login form", "directory": "/frontend"}
	]`

	plans, err := ParsePlans(response)
	if err != nil {
		t.Fatalf("ParsePlans failed: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	if plans[0].Title != "Auth middleware" {
		t.Errorf("expected 'Auth middleware', got %q", plans[0].Title)
	}
	if plans[1].Directory != "/frontend" {
		t.Errorf("expected '/frontend', got %q", plans[1].Directory)
	}
}

func TestParsePlans_MarkdownCodeFence(t *testing.T) {
	response := "```json\n[\n  {\"title\": \"Test\", \"description\": \"desc\", \"directory\": \"/dir\"}\n]\n```"

	plans, err := ParsePlans(response)
	if err != nil {
		t.Fatalf("ParsePlans failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
}

func TestParsePlans_NoJSONArray(t *testing.T) {
	_, err := ParsePlans("no json here")
	if err == nil {
		t.Error("expected error for non-JSON input")
	}
}

func TestParsePlans_EmptyArray(t *testing.T) {
	_, err := ParsePlans("[]")
	if err == nil {
		t.Error("expected error for empty array")
	}
}

func TestPlanner_DecomposeTask(t *testing.T) {
	mockPlans := []SubTaskPlan{
		{Title: "Backend auth", Description: "JWT middleware", Directory: "/backend"},
		{Title: "Frontend auth", Description: "Login page", Directory: "/frontend"},
	}
	response, _ := json.Marshal(mockPlans)

	client := &mockLLMClient{response: string(response)}
	planner := NewPlanner(client, "")

	task := &Task{
		ID:        "task-1",
		Title:     "Add auth",
		Objective: "Add authentication to the app",
		BoundDirs: []BoundDirectory{
			{Path: "/backend", Label: "backend"},
			{Path: "/frontend", Label: "frontend"},
		},
	}

	plans, err := planner.DecomposeTask(context.Background(), task)
	if err != nil {
		t.Fatalf("DecomposeTask failed: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	if plans[0].OrderIndex != 1 {
		t.Errorf("expected OrderIndex 1, got %d", plans[0].OrderIndex)
	}
	if plans[1].OrderIndex != 2 {
		t.Errorf("expected OrderIndex 2, got %d", plans[1].OrderIndex)
	}
}

func TestPlanner_NoBoundDirs(t *testing.T) {
	mockPlans := []SubTaskPlan{
		{Title: "Setup", Description: "Initialize project", Directory: "/suggested-dir"},
	}
	response, _ := json.Marshal(mockPlans)

	client := &mockLLMClient{response: string(response)}
	planner := NewPlanner(client, "")

	task := &Task{
		ID:        "task-2",
		Title:     "Setup project",
		Objective: "Initialize a new project",
		BoundDirs: nil,
	}

	plans, err := planner.DecomposeTask(context.Background(), task)
	if err != nil {
		t.Fatalf("DecomposeTask failed: %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("expected 1 plan, got %d", len(plans))
	}
}

func TestPlanner_NoLLMClient(t *testing.T) {
	planner := NewPlanner(nil, "")
	task := &Task{ID: "t1", Title: "Test"}

	_, err := planner.DecomposeTask(context.Background(), task)
	if err == nil {
		t.Error("expected error when no LLM client")
	}
}

// Integration test with a real HTTP endpoint (requires INTEGRATION=1)
func TestPlanner_Integration(t *testing.T) {
	// Create a mock HTTP server that simulates an OpenAI-compatible endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": `[{"title":"Auth module","description":"Add JWT auth","directory":"/backend"}]`,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create a real OpenAI client pointed at the mock server
	openaiClient := llm.NewOpenAIClient(llm.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	})

	planner := NewPlanner(openaiClient, "")
	task := &Task{
		ID:        "task-1",
		Title:     "Add auth",
		Objective: "Add authentication",
		BoundDirs: []BoundDirectory{
			{Path: "/backend", Label: "backend"},
		},
	}

	plans, err := planner.DecomposeTask(context.Background(), task)
	if err != nil {
		t.Fatalf("DecomposeTask failed: %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("expected 1 plan, got %d", len(plans))
	}
}
