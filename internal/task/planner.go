// Package task provides task management for the TUI coding agent manager.
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codeg/internal/llm"
)

// Planner uses an LLM to decompose tasks into sub-tasks.
type Planner struct {
	client llm.Client
	model  string // optional model override for planning
}

// NewPlanner creates a new task planner.
func NewPlanner(client llm.Client, model string) *Planner {
	return &Planner{client: client, model: model}
}

// DecomposeTask sends the task objective and bound directories to the LLM
// and returns a structured list of sub-task plans.
func (p *Planner) DecomposeTask(ctx context.Context, task *Task) ([]SubTaskPlan, error) {
	if p.client == nil {
		return nil, fmt.Errorf("no LLM client configured for planning")
	}

	systemPrompt := `You are a technical project planner. Given a task objective and a list of available
working directories, decompose the work into ordered sub-tasks.

Each sub-task must:
1. Be self-contained and implementable in a single coding session
2. Be assigned to one of the available directories
3. Have a clear, specific title and a concise description of what to implement
4. Be ordered logically (dependencies first)

Return ONLY a JSON array of objects with these exact keys:
- "title": short title (max 80 chars)
- "description": what needs to be built (2-3 sentences)
- "directory": one of the provided directories

Do not include any text outside the JSON array.`

	// Build directory list for the prompt
	var dirList strings.Builder
	dirList.WriteString("Available directories:\n")
	for i, d := range task.BoundDirs {
		label := d.Label
		if label == "" {
			label = "no label"
		}
		dirList.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, d.Path, label))
	}

	// If no bound dirs, suggest using a common workspace
	if len(task.BoundDirs) == 0 {
		dirList.WriteString("(No directories bound — suggest a reasonable directory name)\n")
	}

	userMsg := fmt.Sprintf("Task: %s\nObjective: %s\n\n%s\n\nDecompose this task into sub-tasks.",
		task.Title, task.Objective, dirList.String())

	messages := []llm.Message{
		{Role: "user", Content: userMsg},
	}

	response, err := p.client.Chat(ctx, systemPrompt, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM decomposition failed: %w", err)
	}

	// Parse JSON response
	plans, err := parsePlans(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w\nResponse was: %s", err, truncate(response, 500))
	}

	// Assign order indices
	for i := range plans {
		plans[i].OrderIndex = i + 1
	}

	return plans, nil
}

// parsePlans extracts SubTaskPlan from LLM JSON response.
func parsePlans(response string) ([]SubTaskPlan, error) {
	// Strip markdown code fences if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Find JSON array boundaries
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	response = response[start : end+1]

	var plans []SubTaskPlan
	if err := json.Unmarshal([]byte(response), &plans); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("LLM returned empty sub-task list")
	}

	return plans, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
