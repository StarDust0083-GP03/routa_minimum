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

// AgentHandle tracks a running coding agent for a sub-task phase.
type AgentHandle struct {
	SessionID string
	SubTaskID string
	TaskID    string
	Phase     string // "coding" or "verifying"
	Directory string
	Cancel    func() error
	Events    <-chan acp.SSEEvent
}

// AgentController manages coding agent lifecycle for sub-tasks.
type AgentController struct {
	runner  agent.AgentRunner
	mu      sync.RWMutex
	handles map[string]*AgentHandle // keyed by subTaskID
}

// NewAgentController creates a new agent controller with the given runner.
func NewAgentController(runner agent.AgentRunner) *AgentController {
	return &AgentController{
		runner:  runner,
		handles: make(map[string]*AgentHandle),
	}
}

// StartSubTaskCoding starts the coding phase for a sub-task.
func (c *AgentController) StartSubTaskCoding(
	ctx context.Context,
	st *task.SubTask,
	taskObj *task.Task,
) (*AgentHandle, error) {
	return c.startPhase(ctx, st, taskObj, task.PhaseCoding, acp.RoleDeveloper, buildCodingPrompt(st, taskObj))
}

// StartSubTaskVerification starts the verification phase for a sub-task.
func (c *AgentController) StartSubTaskVerification(
	ctx context.Context,
	st *task.SubTask,
	taskObj *task.Task,
) (*AgentHandle, error) {
	return c.startPhase(ctx, st, taskObj, task.PhaseVerifying, acp.RoleGate, buildVerificationPrompt(st, taskObj))
}

// startPhase is the internal phase launcher.
func (c *AgentController) startPhase(
	ctx context.Context,
	st *task.SubTask,
	taskObj *task.Task,
	phase string,
	role acp.AgentRole,
	prompt string,
) (*AgentHandle, error) {
	cwd := st.Directory
	if cwd == "" && len(taskObj.BoundDirs) > 0 {
		cwd = taskObj.BoundDirs[0].Path
	}
	if cwd == "" {
		return nil, fmt.Errorf("no directory for sub-task %s", st.ID)
	}

	// Check if agent already running for this sub-task
	c.mu.RLock()
	if _, exists := c.handles[st.ID]; exists {
		c.mu.RUnlock()
		return nil, fmt.Errorf("agent already running for sub-task %s", st.ID)
	}
	c.mu.RUnlock()

	result, err := c.runner.Start(ctx, cwd, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to start %s agent: %w", phase, err)
	}

	handle := &AgentHandle{
		SessionID: result.SessionID,
		SubTaskID: st.ID,
		TaskID:    taskObj.ID,
		Phase:     phase,
		Directory: cwd,
		Events:    result.Events,
		Cancel:    result.Cancel,
	}

	c.mu.Lock()
	c.handles[st.ID] = handle
	c.mu.Unlock()

	return handle, nil
}

// CancelAgent cancels a running agent for a sub-task.
func (c *AgentController) CancelAgent(subTaskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	handle, exists := c.handles[subTaskID]
	if !exists {
		return fmt.Errorf("no agent running for sub-task %s", subTaskID)
	}

	if err := handle.Cancel(); err != nil {
		return fmt.Errorf("failed to cancel agent: %w", err)
	}

	delete(c.handles, subTaskID)
	return nil
}

// GetHandle returns the agent handle for a sub-task, or nil.
func (c *AgentController) GetHandle(subTaskID string) *AgentHandle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.handles[subTaskID]
}

// RemoveHandle removes the agent handle for a sub-task.
func (c *AgentController) RemoveHandle(subTaskID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.handles, subTaskID)
}

// RunningSubTasks returns the IDs of sub-tasks with active agents.
func (c *AgentController) RunningSubTasks() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var ids []string
	for id := range c.handles {
		ids = append(ids, id)
	}
	return ids
}

// IsRunning returns true if a sub-task has an active agent.
func (c *AgentController) IsRunning(subTaskID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.handles[subTaskID]
	return exists
}

// buildCodingPrompt creates the prompt for the coding phase.
func buildCodingPrompt(st *task.SubTask, t *task.Task) string {
	return fmt.Sprintf(`You are a coding agent working on a sub-task of a larger project.

Task: %s
Sub-Task: %s
Description: %s

Work in the current directory. Implement the sub-task described above.
Write clean, well-documented code. Follow existing patterns in the codebase.
When you're done, verify your implementation compiles and tests pass.`, t.Title, st.Title, st.Description)
}

// buildVerificationPrompt creates the prompt for the verification phase.
func buildVerificationPrompt(st *task.SubTask, t *task.Task) string {
	return fmt.Sprintf(`You are a verification agent (GATE role). Your job is to verify the implementation
of a sub-task that was just coded by another agent. Be thorough and skeptical.

Task: %s
Sub-Task: %s
Expected Implementation: %s

Please verify:
1. All files mentioned in the coding session exist and are correct
2. The implementation matches the sub-task description
3. Run any available tests (cargo test, go test, npm test, etc.)
4. Check for edge cases, error handling, and security issues
5. Report any issues found — do NOT fix them, just report

If everything is correct, confirm with: "VERIFICATION PASSED: <brief summary>"
If there are issues, report with: "VERIFICATION FAILED: <detailed issues>"`, t.Title, st.Title, st.Description)
}
