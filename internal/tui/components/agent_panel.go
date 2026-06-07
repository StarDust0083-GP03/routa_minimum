// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	agentPanelTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6E3A1")).
				Bold(true).
				PaddingLeft(1)

	agentChunkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CDD6F4"))

	agentToolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89B4FA"))
)

// AgentEventMsg represents a streaming event from a coding agent.
type AgentEventMsg struct {
	TaskID    string
	EventType string
	Text      string
}

// AgentDoneMsg indicates the agent has completed.
type AgentDoneMsg struct {
	TaskID string
}

// AgentPanel displays real-time coding agent output.
type AgentPanel struct {
	viewport     viewport.Model
	content      string
	taskID       string
	taskName     string
	isActive     bool
	width        int
	height       int
}

// NewAgentPanel creates a new agent output panel.
func NewAgentPanel(width, height int) AgentPanel {
	vp := viewport.New(width, height)
	return AgentPanel{
		viewport: vp,
		width:    width,
		height:   height,
	}
}

// SetActive sets the agent panel to show output for a task.
// Does not clear content if already active for the same task.
func (ap *AgentPanel) SetActive(taskID, taskName string) {
	if ap.taskID != taskID {
		ap.content = ""
		ap.viewport.SetContent("")
	}
	ap.taskID = taskID
	ap.taskName = taskName
	ap.isActive = true
}

// SetInactive marks the agent as no longer running.
func (ap *AgentPanel) SetInactive() {
	ap.isActive = false
}

// IsActive returns whether an agent is running.
func (ap *AgentPanel) IsActive() bool {
	return ap.isActive
}

// Append appends a line of output from the agent.
func (ap *AgentPanel) Append(text string) {
	if ap.content == "" {
		ap.content = text
	} else {
		ap.content += "\n" + text
	}
	ap.renderContent()
}

// HandleEvent processes an agent event message.
func (ap *AgentPanel) HandleEvent(event AgentEventMsg) {
	switch event.EventType {
	case "agent_message_chunk":
		ap.Append(agentChunkStyle.Render(event.Text))
	case "tool_call":
		ap.Append(agentToolStyle.Render("[tool] " + event.Text))
	case "tool_call_update":
		ap.Append(agentChunkStyle.Render("  " + event.Text))
	case "process_output":
		ap.Append(agentChunkStyle.Render(event.Text))
	case "turn_complete":
		ap.Append(lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("--- turn complete ---"))
	case "error":
		ap.Append(lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("ERROR: " + event.Text))
	}
}

// Update handles messages.
func (ap *AgentPanel) Update(msg tea.Msg) (AgentPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case AgentEventMsg:
		// Auto-activate panel on first event for this task
		if !ap.isActive || ap.taskID != msg.TaskID {
			ap.taskID = msg.TaskID
			ap.taskName = ""
			ap.isActive = true
		}
		ap.HandleEvent(msg)
		return *ap, nil
	case AgentDoneMsg:
		if msg.TaskID == ap.taskID {
			ap.isActive = false
			ap.Append(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6E3A1")).
				Bold(true).
				Render("Agent completed."))
		}
		return *ap, nil
	}
	return *ap, nil
}

// View renders the agent panel.
func (ap *AgentPanel) View() string {
	return ap.viewport.View()
}

// Resize updates the component dimensions.
func (ap *AgentPanel) Resize(width, height int) {
	ap.width = width
	ap.height = height
	ap.viewport.Width = width
	ap.viewport.Height = height
	ap.renderContent()
}

func (ap *AgentPanel) renderContent() {
	var header string
	if ap.isActive {
		header = agentPanelTitleStyle.Render("Agent Output") + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("(running...)")
	} else if ap.content != "" {
		header = agentPanelTitleStyle.Render("Agent Output")
	} else {
		header = agentPanelTitleStyle.Render("Agent Output")
	}

	fullContent := header + "\n\n"
	if ap.content == "" {
		fullContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585B70")).
			Render("  No agent output. Press 'r' on a task to start a coding agent.")
	} else {
		fullContent += ap.content
	}

	ap.viewport.SetContent(fullContent)
	ap.viewport.GotoBottom()
}
