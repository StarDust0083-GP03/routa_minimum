// Package opencode implements AgentRunner using opencode CLI as a subprocess.
// Uses "opencode run" which outputs JSON lines on stdout.
package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"codeg/internal/acp"
	"codeg/internal/agent"
)

// Runner implements agent.AgentRunner by spawning opencode as a subprocess.
type Runner struct {
	binPath string // path to opencode binary
	model   string // optional model override
}

// NewRunner creates a new opencode runner.
// serverURL is kept for interface compatibility but not used in subprocess mode.
func NewRunner(serverURL string) *Runner {
	return &Runner{binPath: "opencode"}
}

// Start launches "opencode run --dir <cwd> --format json <prompt>".
func (r *Runner) Start(ctx context.Context, cwd, prompt string, role acp.AgentRole) (*agent.StartResult, error) {
	_ = role // opencode uses its own provider config for model selection

	args := []string{
		"run",
		"--dir", cwd,
		"--format", "json",
	}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, r.binPath, args...)
	cmd.Dir = cwd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start opencode: %w", err)
	}

	ch := make(chan acp.SSEEvent, 64)
	var once sync.Once

	cancelFunc := func() error {
		var err error
		once.Do(func() {
			if cmd.Process != nil {
				err = cmd.Process.Kill()
			}
		})
		return err
	}

	// Read the first JSON line to get the session ID
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var sessionID string
	if scanner.Scan() {
		firstLine := scanner.Bytes()
		var raw map[string]interface{}
		if err := json.Unmarshal(firstLine, &raw); err == nil {
			if sid, ok := raw["sessionID"].(string); ok {
				sessionID = sid
			}
		}
		ch <- parseOpenCodeEvent(raw)
	}

	// Read stderr in background
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			ch <- acp.SSEEvent{
				Type: acp.EventProcessOutput,
				Data: map[string]interface{}{"text": sc.Text()},
			}
		}
	}()

	// Read remaining stdout events in background
	go func() {
		defer close(ch)
		defer cmd.Wait()

		for scanner.Scan() {
			line := scanner.Bytes()
			var raw map[string]interface{}
			if err := json.Unmarshal(line, &raw); err != nil {
				ch <- acp.SSEEvent{
					Type: acp.EventMessageChunk,
					Data: map[string]interface{}{"text": string(line)},
				}
				continue
			}

			event := parseOpenCodeEvent(raw)
			ch <- event

			if event.Type == acp.EventTurnComplete {
				return
			}
		}

		ch <- acp.SSEEvent{
			Type: acp.EventTurnComplete,
			Data: map[string]interface{}{},
		}
	}()

	return &agent.StartResult{
		Events:    ch,
		Cancel:    cancelFunc,
		SessionID: sessionID,
	}, nil
}

// Resume is not supported in subprocess mode.
func (r *Runner) Resume(ctx context.Context, sessionID string) (*agent.StartResult, error) {
	return nil, fmt.Errorf("resume not supported in subprocess mode")
}

// Load is not supported in subprocess mode.
func (r *Runner) Load(ctx context.Context, sessionID string) (*acp.SessionResponse, error) {
	return nil, fmt.Errorf("load not supported in subprocess mode")
}

// parseOpenCodeEvent converts an opencode JSON event to an ACP SSEEvent.
func parseOpenCodeEvent(raw map[string]interface{}) acp.SSEEvent {
	typ, _ := raw["type"].(string)
	part, _ := raw["part"].(map[string]interface{})

	event := acp.SSEEvent{Type: typ, Data: raw}

	switch typ {
	case "text":
		event.Type = acp.EventMessageChunk
		text := ""
		if part != nil {
			if t, ok := part["text"].(string); ok {
				text = t
			}
		}
		event.Data = map[string]interface{}{"text": text}

	case "tool_use":
		// opencode v1.14+ uses unified "tool_use" events
		event.Type = acp.EventToolCall
		if part != nil {
			data := map[string]interface{}{}
			if toolName, ok := part["tool"].(string); ok {
				data["name"] = toolName
			}
			// Extract input/output from state if present
			if state, ok := part["state"].(map[string]interface{}); ok {
				if input, ok := state["input"].(map[string]interface{}); ok {
					if cmd, ok := input["command"].(string); ok {
						data["args"] = cmd
					}
				}
				if output, ok := state["output"].(string); ok {
					// Also send a tool_update with the output
					data["output"] = truncateStr(output, 2000)
				}
			}
			event.Data = data
		}

	case "tool_call":
		if part != nil {
			data := map[string]interface{}{}
			if name, ok := part["name"].(string); ok {
				data["name"] = name
			}
			if args, ok := part["args"]; ok {
				data["args"] = args
			}
			event.Data = data
		}

	case "tool_result":
		event.Type = acp.EventToolUpdate
		if part != nil {
			if result, ok := part["result"]; ok {
				event.Data = map[string]interface{}{"output": truncateStr(fmt.Sprint(result), 2000)}
			} else if text, ok := part["text"].(string); ok {
				event.Data = map[string]interface{}{"output": text}
			}
		}

	case "step_start":
		event.Type = acp.EventMessageChunk
		event.Data = map[string]interface{}{"text": "--- agent started ---"}

	case "step_finish":
		event.Type = acp.EventTurnComplete
		if part != nil {
			if tokens, ok := part["tokens"].(map[string]interface{}); ok {
				event.Data = map[string]interface{}{"tokens": tokens}
			}
		}

	case "error":
		msg := ""
		if part != nil {
			if m, ok := part["text"].(string); ok {
				msg = m
			}
		}
		if msg == "" {
			if m, ok := raw["message"].(string); ok {
				msg = m
			}
		}
		event.Data = map[string]interface{}{"message": msg}
	}

	return event
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

