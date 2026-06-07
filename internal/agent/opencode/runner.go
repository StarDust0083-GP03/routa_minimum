// Package opencode implements AgentRunner using the opencode CLI as a subprocess.
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
	binPath string // path to opencode binary, defaults to "opencode"
	model   string // optional model override
	agentID string // optional agent override
}

// NewRunner creates a new OpenCode runner.
func NewRunner(binPath, model, agentID string) *Runner {
	if binPath == "" {
		binPath = "opencode"
	}
	return &Runner{binPath: binPath, model: model, agentID: agentID}
}

// Start launches opencode run --dir <cwd> --format json "<prompt>".
func (r *Runner) Start(ctx context.Context, cwd, prompt string) (*agent.StartResult, error) {
	args := []string{
		"run",
		"--dir", cwd,
		"--format", "json",
	}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	if r.agentID != "" {
		args = append(args, "--agent", r.agentID)
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

	// Read the first JSON line synchronously to get the session ID.
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
			// Send the first event
			ch <- r.parseOpenCodeEvent(raw)
		}
	}

	// Read stderr in background
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			ch <- acp.SSEEvent{
				Type: "process_output",
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
					Type: "agent_message_chunk",
					Data: map[string]interface{}{"text": string(line)},
				}
				continue
			}

			event := r.parseOpenCodeEvent(raw)
			ch <- event

			if event.Type == "turn_complete" {
				return
			}
		}

		ch <- acp.SSEEvent{
			Type: "turn_complete",
			Data: map[string]interface{}{},
		}
	}()

	return &agent.StartResult{
		Events:    ch,
		Cancel:    cancelFunc,
		SessionID: sessionID,
	}, nil
}

// parseOpenCodeEvent converts an opencode JSON event to an ACP SSEEvent.
func (r *Runner) parseOpenCodeEvent(raw map[string]interface{}) acp.SSEEvent {
	typ, _ := raw["type"].(string)
	part, _ := raw["part"].(map[string]interface{})

	event := acp.SSEEvent{Type: typ, Data: raw}

	switch typ {
	case "text":
		event.Type = "agent_message_chunk"
		text := ""
		if part != nil {
			if t, ok := part["text"].(string); ok {
				text = t
			}
		}
		event.Data = map[string]interface{}{"text": text}

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
		event.Type = "tool_call_update"
		if part != nil {
			if result, ok := part["result"]; ok {
				event.Data = map[string]interface{}{"output": fmt.Sprint(result)}
			} else if text, ok := part["text"].(string); ok {
				event.Data = map[string]interface{}{"output": text}
			}
		}

	case "step_start":
		event.Type = "agent_message_chunk"
		event.Data = map[string]interface{}{"text": "--- agent started ---"}

	case "step_finish":
		event.Type = "turn_complete"
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
