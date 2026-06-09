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
	agentCodingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F9E2AF"))
	agentVerifyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94E2D5"))
)

type AgentEventMsg struct {
	TaskID    string
	SubTaskID string
	Phase     string
	EventType string
	Text      string
}

type AgentDoneMsg struct {
	TaskID    string
	SubTaskID string
	Phase     string
}

type AgentPanel struct {
	viewport  viewport.Model
	content   string // plain string to avoid strings.Builder copy-on-write panic
	taskID    string
	subTaskID string
	taskName  string
	phase     string
	isActive  bool
	width     int
	height    int
}

func NewAgentPanel(width, height int) AgentPanel {
	vp := viewport.New(width, height)
	return AgentPanel{viewport: vp, width: width, height: height}
}

func (ap *AgentPanel) SetActive(taskID, subTaskID, taskName, phase string) {
	// Only clear content when switching to a different sub-task
	if ap.subTaskID != subTaskID {
		ap.content = ""
		ap.viewport.SetContent("")
	}
	ap.taskID = taskID
	ap.subTaskID = subTaskID
	ap.taskName = taskName
	ap.phase = phase
	ap.isActive = true
}

func (ap *AgentPanel) SetInactive() { ap.isActive = false }
func (ap *AgentPanel) GetFullOutput() string { return ap.content }
func (ap *AgentPanel) IsActive() bool { return ap.isActive }
func (ap *AgentPanel) Phase() string { return ap.phase }

// LoadOutput restores previously saved agent output for a sub-task.
func (ap *AgentPanel) LoadOutput(output string) {
	ap.content = output
	ap.renderContent()
}

func (ap *AgentPanel) Append(text string) {
	if ap.content != "" {
		ap.content += "\n"
	}
	ap.content += text
	ap.renderContent()
}

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

func (ap *AgentPanel) View() string { return ap.viewport.View() }

func (ap *AgentPanel) Resize(width, height int) {
	ap.width = width
	ap.height = height
	ap.viewport.Width = width
	ap.viewport.Height = height
	ap.renderContent()
}

func (ap *AgentPanel) renderContent() {
	var statusText string
	var header string
	if ap.isActive {
		prefix := ap.taskName
		if prefix == "" {
			prefix = "Agent Output"
		}
		if ap.phase == "verifying" {
			statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#94E2D5")).Render("(verifying...)")
		} else {
			statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("(coding...)")
		}
		header = agentPanelTitleStyle.Render(prefix) + " " + statusText
	} else if ap.content != "" {
		statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6ADC8")).Render("(done)")
		header = agentPanelTitleStyle.Render("Agent Output") + " " + statusText
	} else {
		statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("(idle)")
		header = agentPanelTitleStyle.Render("Agent Output") + " " + statusText
	}
	fullContent := header + "\n\n"
	if ap.content == "" {
		fullContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585B70")).
			Render("  No agent output. Press 'r' on a sub-task to start the coding agent.")
	} else {
		fullContent += ap.content
	}
	ap.viewport.SetContent(fullContent)
	ap.viewport.GotoBottom()
}
