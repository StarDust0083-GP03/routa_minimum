// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"
	"strings"

	"codeg/internal/task"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	taskListTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89B4FA")).
				Bold(true).
				PaddingLeft(1)

	taskItemStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	taskItemSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#45475A")).
				PaddingLeft(1)

	taskPriorityHigh = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F38BA8"))

	taskPriorityMedium = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F9E2AF"))

	taskPriorityLow = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#585B70"))
)

// TaskListMsg is sent when a task is selected.
type TaskListMsg struct {
	Task    *task.Task
	Index   int
}

// TaskList renders the selectable task list and handles navigation.
type TaskList struct {
	viewport      viewport.Model
	tasks         []*task.Task
	selectedIndex int
	width         int
	height        int
}

// NewTaskList creates a new task list component.
func NewTaskList(width, height int) TaskList {
	vp := viewport.New(width, height)
	return TaskList{
		viewport:      vp,
		selectedIndex: 0,
		width:         width,
		height:        height,
	}
}

// SetTasks updates the task list content.
func (tl *TaskList) SetTasks(tasks []*task.Task) {
	tl.tasks = tasks
	if tl.selectedIndex >= len(tasks) && len(tasks) > 0 {
		tl.selectedIndex = len(tasks) - 1
	}
	tl.renderContent()
}

// SelectedTask returns the currently selected task.
func (tl *TaskList) SelectedTask() *task.Task {
	if tl.selectedIndex < 0 || tl.selectedIndex >= len(tl.tasks) {
		return nil
	}
	return tl.tasks[tl.selectedIndex]
}

// SelectedIndex returns the current selection index.
func (tl *TaskList) SelectedIndex() int {
	return tl.selectedIndex
}

// Update handles navigation messages.
func (tl *TaskList) Update(msg tea.Msg) (TaskList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if tl.selectedIndex > 0 {
				tl.selectedIndex--
				tl.renderContent()
			}
		case "down", "j":
			if tl.selectedIndex < len(tl.tasks)-1 {
				tl.selectedIndex++
				tl.renderContent()
			}
		case "enter":
			if t := tl.SelectedTask(); t != nil {
				return *tl, func() tea.Msg {
					return TaskListMsg{Task: t, Index: tl.selectedIndex}
				}
			}
		}
	}
	return *tl, nil
}

// View renders the task list.
func (tl *TaskList) View() string {
	return tl.viewport.View()
}

// Resize updates the component dimensions.
func (tl *TaskList) Resize(width, height int) {
	tl.width = width
	tl.height = height
	tl.viewport.Width = width
	tl.viewport.Height = height
	tl.renderContent()
}

func (tl *TaskList) renderContent() {
	var sb strings.Builder

	// Title
	sb.WriteString(taskListTitleStyle.Render("Tasks"))
	sb.WriteString("\n\n")

	if len(tl.tasks) == 0 {
		sb.WriteString(taskItemStyle.Render("  No tasks. Press 'n' to create one."))
	} else {
		for i, t := range tl.tasks {
			var row string
			dot := StatusDot(t.Status)
			title := truncate(t.Title, tl.width-15)

			if i == tl.selectedIndex {
				row = taskItemSelectedStyle.Width(tl.width - 1).Render(
					fmt.Sprintf(" %s %s %s", dot, priorityIcon(t.Priority), title),
				)
			} else {
				row = taskItemStyle.Render(
					fmt.Sprintf(" %s %s %s", dot, priorityIcon(t.Priority), title),
				)
			}
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}

	tl.viewport.SetContent(sb.String())
}

func priorityIcon(p task.TaskPriority) string {
	switch p {
	case task.PriorityHigh:
		return taskPriorityHigh.Render("▲")
	case task.PriorityMedium:
		return taskPriorityMedium.Render("●")
	case task.PriorityLow:
		return taskPriorityLow.Render("▼")
	default:
		return " "
	}
}

// StatusDot returns a colored dot for the given status.
func StatusDot(status task.TaskStatus) string {
	switch status {
	case task.TaskPending:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("●")
	case task.TaskInProgress:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("●")
	case task.TaskReview:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#89DCEB")).Render("●")
	case task.TaskCompleted:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("●")
	case task.TaskBlocked:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("●")
	case task.TaskCancelled:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("✕")
	default:
		return "●"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
