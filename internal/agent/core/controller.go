// Package core provides agent lifecycle management for the TUI coding agent manager.
package core

import (
	"context"
	"fmt"
	"sync"

	"codeg/internal/acp"
	"codeg/internal/agent"
	"codeg/internal/task"
)

// AgentHandle tracks a running coding agent.
type AgentHandle struct {
	SessionID string
	TaskID    string
	BoundDir  string
	Cancel    func() error
	Events    <-chan acp.SSEEvent
}

// AgentController manages coding agent lifecycle.
type AgentController struct {
	runner  agent.AgentRunner
	mu      sync.RWMutex
	handles map[string]*AgentHandle
}

// NewAgentController creates a new agent controller with the given runner.
func NewAgentController(runner agent.AgentRunner) *AgentController {
	return &AgentController{
		runner:  runner,
		handles: make(map[string]*AgentHandle),
	}
}

// StartAgent spawns a coding agent for a task in a specific bound directory.
func (c *AgentController) StartAgent(
	ctx context.Context,
	t *task.Task,
	boundDir string,
	prompt string,
) (*AgentHandle, error) {
	// Validate bound directory
	found := false
	for _, d := range t.BoundDirs {
		if d.Path == boundDir {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("directory %s is not bound to task %s", boundDir, t.ID)
	}

	// Check if agent already running
	c.mu.RLock()
	if _, exists := c.handles[t.ID]; exists {
		c.mu.RUnlock()
		return nil, fmt.Errorf("agent already running for task %s", t.ID)
	}
	c.mu.RUnlock()

	result, err := c.runner.Start(ctx, boundDir, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	handle := &AgentHandle{
		SessionID: result.SessionID,
		TaskID:    t.ID,
		BoundDir:  boundDir,
		Events:    result.Events,
		Cancel:    result.Cancel,
	}

	c.mu.Lock()
	c.handles[t.ID] = handle
	c.mu.Unlock()

	return handle, nil
}

// CancelAgent cancels a running agent for the given task.
func (c *AgentController) CancelAgent(taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	handle, exists := c.handles[taskID]
	if !exists {
		return fmt.Errorf("no agent running for task %s", taskID)
	}

	if err := handle.Cancel(); err != nil {
		return fmt.Errorf("failed to cancel agent: %w", err)
	}

	delete(c.handles, taskID)
	return nil
}

// GetHandle returns the agent handle for a task, or nil.
func (c *AgentController) GetHandle(taskID string) *AgentHandle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.handles[taskID]
}

// RemoveHandle removes the agent handle for a task.
func (c *AgentController) RemoveHandle(taskID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.handles, taskID)
}

// RunningTasks returns the IDs of tasks with active agents.
func (c *AgentController) RunningTasks() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var ids []string
	for id := range c.handles {
		ids = append(ids, id)
	}
	return ids
}
