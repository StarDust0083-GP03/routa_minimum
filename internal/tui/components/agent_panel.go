// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"strings"

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

	agentCodingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F9E2AF"))

	agentVerifyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94E2D5"))
)

// AgentEventMsg represents a streaming event from a coding agent.
type AgentEventMsg struct {
	TaskID    string
	SubTaskID string
	Phase     string // "coding" or "verifying"
	EventType string
	Text      string
}

// AgentDoneMsg indicates the agent has completed a phase.
type AgentDoneMsg struct {
	TaskID    string
	SubTaskID string
	Phase     string
}

// AgentPanel displays real-time coding agent output.
type AgentPanel struct {
	viewport  viewport.Model
	content   strings.Builder
	taskID    string
	subTaskID string
	taskName  string
	phase     string
	isActive  bool
	width     int
	height    int
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

// SetActive sets the agent panel to show output for a sub-task phase.
func (ap *AgentPanel) SetActive(taskID, subTaskID, taskName, phase string) {
	if ap.subTaskID != subTaskID || ap.phase != phase {
		ap.content.Reset()
		ap.viewport.SetContent("")
	}
	ap.taskID = taskID
	ap.subTaskID = subTaskID
	ap.taskName = taskName
	ap.phase = phase
	ap.isActive = true
}

// SetInactive marks the agent as no longer running.
func (ap *AgentPanel) SetInactive() {
	ap.isActive = false
}

// GetFullOutput returns the complete accumulated agent output text.
func (ap *AgentPanel) GetFullOutput() string {
	return ap.content.String()
}

// IsActive returns whether an agent is running.
func (ap *AgentPanel) IsActive() bool {
	return ap.isActive
}

// Phase returns the current phase.
func (ap *AgentPanel) Phase() string {
	return ap.phase
}

// Append appends a line of output from the agent.
func (ap *AgentPanel) Append(text string) {
	if ap.content.Len() > 0 {
		ap.content.WriteString("\n")
	}
	ap.content.WriteString(text)
	ap.renderContent()
}

// HandleEvent processes an agent event message.
func (ap *AgentPanel) HandleEvent(event AgentEventMsg) {
	var phasePrefix string
	switch event.Phase {
	case "coding":
		phasePrefix = agentCodingStyle.Render("[coding] ")
	case "verifying":
		phasePrefix = agentVerifyStyle.Render("[verify] ")
	}

	switch event.EventType {
	case "agent_message_chunk":
		ap.Append(phasePrefix + agentChunkStyle.Render(event.Text))
	case "tool_call":
		ap.Append(phasePrefix + agentToolStyle.Render("[tool] "+event.Text))
	case "tool_call_update":
		ap.Append(phasePrefix + agentChunkStyle.Render("  "+event.Text))
	case "process_output":
		ap.Append(phasePrefix + agentChunkStyle.Render(event.Text))
	case "turn_complete":
		ap.Append(phasePrefix + lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("--- phase complete ---"))
	case "error":
		ap.Append(phasePrefix + lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("ERROR: "+event.Text))
	}
}

// Update handles messages.
func (ap *AgentPanel) Update(msg tea.Msg) (AgentPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case AgentEventMsg:
		if !ap.isActive || ap.subTaskID != msg.SubTaskID {
			ap.subTaskID = msg.SubTaskID
			ap.taskID = msg.TaskID
			ap.phase = msg.Phase
			ap.taskName = ""
			ap.isActive = true
		}
		ap.HandleEvent(msg)
		return *ap, nil
	case AgentDoneMsg:
		if msg.SubTaskID == ap.subTaskID {
			ap.isActive = false
			ap.Append(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6E3A1")).
				Bold(true).
				Render("--- phase complete ---"))
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
	var statusText string
	if ap.isActive {
		if ap.phase == "verifying" {
			statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#94E2D5")).Render("(verifying...)")
		} else {
			statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("(coding...)")
		}
	} else if ap.content.Len() > 0 {
		statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6ADC8")).Render("(done)")
	} else {
		statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("(idle)")
	}

	header := agentPanelTitleStyle.Render("Agent Output") + " " + statusText

	fullContent := header + "\n\n"
	if ap.content.Len() == 0 {
		fullContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585B70")).
			Render("  No agent output. Press 'r' on a sub-task to start the coding agent.")
	} else {
		fullContent += ap.content.String()
	}

	ap.viewport.SetContent(fullContent)
	ap.viewport.GotoBottom()
}
