// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	createFormStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#89B4FA")).
			Padding(1, 2)

	createFormTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBA6F7")).
				Bold(true)

	createFormLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89B4FA"))
)

// CreateTaskDoneMsg indicates the task creation form was submitted or cancelled.
type CreateTaskDoneMsg struct {
	Title     string
	Objective string
	Cancelled bool
}

// CreateTaskForm is the modal form for creating a new task.
type CreateTaskForm struct {
	titleInput     textinput.Model
	objectiveInput textinput.Model
	focusIndex     int
	width          int
}

// NewCreateTaskForm creates a new task creation form.
func NewCreateTaskForm() CreateTaskForm {
	ti := textinput.New()
	ti.Placeholder = "Task title..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	oi := textinput.New()
	oi.Placeholder = "What should be done? (optional)"
	oi.CharLimit = 500
	oi.Width = 50

	return CreateTaskForm{
		titleInput:     ti,
		objectiveInput: oi,
		focusIndex:     0,
		width:          50,
	}
}

// Init initializes the form.
func (f CreateTaskForm) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles key events for the form.
func (f CreateTaskForm) Update(msg tea.Msg) (CreateTaskForm, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			f.focusIndex++
			if f.focusIndex > 1 {
				f.focusIndex = 0
			}
			f.updateFocus()
		case "shift+tab":
			f.focusIndex--
			if f.focusIndex < 0 {
				f.focusIndex = 1
			}
			f.updateFocus()
		case "enter":
			if f.focusIndex == 1 {
				// Submit
				return f, func() tea.Msg {
					return CreateTaskDoneMsg{
						Title:     f.titleInput.Value(),
						Objective: f.objectiveInput.Value(),
					}
				}
			}
			// Move to next field
			f.focusIndex = 1
			f.updateFocus()
		case "esc":
			return f, func() tea.Msg {
				return CreateTaskDoneMsg{Cancelled: true}
			}
		}
	}

	// Update focused input
	if f.focusIndex == 0 {
		var cmd tea.Cmd
		f.titleInput, cmd = f.titleInput.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		f.objectiveInput, cmd = f.objectiveInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return f, tea.Batch(cmds...)
}

func (f *CreateTaskForm) updateFocus() {
	if f.focusIndex == 0 {
		f.titleInput.Focus()
		f.objectiveInput.Blur()
	} else {
		f.titleInput.Blur()
		f.objectiveInput.Focus()
	}
}

// View renders the form.
func (f CreateTaskForm) View() string {
	title := createFormTitleStyle.Render("New Task")
	content := title + "\n\n"
	content += createFormLabelStyle.Render("Title:") + "\n"
	content += f.titleInput.View() + "\n\n"
	content += createFormLabelStyle.Render("Objective:") + "\n"
	content += f.objectiveInput.View() + "\n\n"

	help := CreateFormHelp()
	var helpParts []string
	for _, h := range help {
		helpParts = append(helpParts, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89B4FA")).Bold(true).Render(h[0])+" "+h[1])
	}
	content += lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render(
		joinHelp(helpParts),
	)

	return createFormStyle.Render(content)
}

func joinHelp(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "  │  "
		}
		result += p
	}
	return result
}
