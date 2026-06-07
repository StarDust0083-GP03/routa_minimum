// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"
	"strings"

	"codeg/internal/task"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	detailTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBA6F7")).
				Bold(true).
				PaddingLeft(1)

	detailSectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89B4FA")).
				Bold(true)

	detailTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CDD6F4"))

	detailPathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E3A1"))
)

// TaskDetail renders the detail view for a selected task.
type TaskDetail struct {
	viewport viewport.Model
	task     *task.Task
	width    int
	height   int
}

// NewTaskDetail creates a new task detail component.
func NewTaskDetail(width, height int) TaskDetail {
	vp := viewport.New(width, height)
	return TaskDetail{
		viewport: vp,
		width:    width,
		height:   height,
	}
}

// SetTask updates the displayed task.
func (td *TaskDetail) SetTask(t *task.Task) {
	td.task = t
	td.renderContent()
}

// Task returns the currently displayed task.
func (td *TaskDetail) Task() *task.Task {
	return td.task
}

// View renders the detail panel.
func (td *TaskDetail) View() string {
	return td.viewport.View()
}

// Resize updates the component dimensions.
func (td *TaskDetail) Resize(width, height int) {
	td.width = width
	td.height = height
	td.viewport.Width = width
	td.viewport.Height = height
	td.renderContent()
}

func (td *TaskDetail) renderContent() {
	var sb strings.Builder

	if td.task == nil {
		sb.WriteString(detailTitleStyle.Render("Task Detail"))
		sb.WriteString("\n\n")
		sb.WriteString(detailTextStyle.Render("  Select a task to view details."))
	} else {
		t := td.task

		// Title and status
		sb.WriteString(detailTitleStyle.Render(t.Title))
		sb.WriteString("\n\n")

		sb.WriteString(detailSectionStyle.Render("Status: "))
		sb.WriteString(StyleStatusLabel(string(t.Status)))
		sb.WriteString("\n")

		sb.WriteString(detailSectionStyle.Render("Priority: "))
		sb.WriteString(string(t.Priority))
		sb.WriteString("\n\n")

		// Objective
		if t.Objective != "" {
			sb.WriteString(detailSectionStyle.Render("Objective"))
			sb.WriteString("\n")
			sb.WriteString(detailTextStyle.Render(wordWrap(t.Objective, td.width-4)))
			sb.WriteString("\n\n")
		}

		// Bound directories
		sb.WriteString(detailSectionStyle.Render("Bound Directories"))
		sb.WriteString(fmt.Sprintf(" (%d)", len(t.BoundDirs)))
		sb.WriteString("\n")
		if len(t.BoundDirs) == 0 {
			sb.WriteString(detailTextStyle.Render("  None. Press 'b' to bind."))
		} else {
			for _, d := range t.BoundDirs {
				label := d.Label
				if label == "" {
					label = "--"
				}
				sb.WriteString(fmt.Sprintf("  %s ", detailPathStyle.Render(d.Path)))
				sb.WriteString(fmt.Sprintf("(%s)", detailTextStyle.Render(label)))
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")

		// Sessions
		sb.WriteString(detailSectionStyle.Render("Sessions"))
		sb.WriteString(fmt.Sprintf(" (%d)", len(t.SessionIDs)))
		sb.WriteString("\n")
		if len(t.SessionIDs) == 0 {
			sb.WriteString(detailTextStyle.Render("  No sessions linked."))
		} else {
			for _, sid := range t.SessionIDs {
				sb.WriteString(fmt.Sprintf("  • %s\n", sid[:12]))
			}
		}
		sb.WriteString("\n")

		// Summary
		if t.Summary != "" {
			sb.WriteString(detailSectionStyle.Render("Summary"))
			sb.WriteString("\n")
			sb.WriteString(detailTextStyle.Render(wordWrap(t.Summary, td.width-4)))
			sb.WriteString("\n\n")
		}

		// Labels
		if len(t.Labels) > 0 {
			sb.WriteString(detailSectionStyle.Render("Labels: "))
			sb.WriteString(strings.Join(t.Labels, ", "))
			sb.WriteString("\n")
		}
	}

	td.viewport.SetContent(sb.String())
}

// StatusLabel returns a styled label for the status.
func StatusLabel(status string) string {
	switch status {
	case "pending":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("PENDING")
	case "in_progress":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("IN PROGRESS")
	case "review":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#89DCEB")).Render("REVIEW")
	case "completed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("COMPLETED")
	case "blocked":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("BLOCKED")
	case "cancelled":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("CANCELLED")
	default:
		return status
	}
}

// StyleStatusLabel returns styled label with lipgloss.Style.
func StyleStatusLabel(status string) string {
	s := StatusLabel(status)
	return s
}

func wordWrap(text string, width int) string {
	if width < 20 {
		width = 20
	}
	var result strings.Builder
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		for len(line) > width {
			pos := strings.LastIndex(line[:width], " ")
			if pos < 0 {
				pos = width
			}
			result.WriteString("  ")
			result.WriteString(line[:pos])
			result.WriteString("\n")
			line = line[pos:]
			if len(line) > 0 && line[0] == ' ' {
				line = line[1:]
			}
		}
		if len(line) > 0 {
			result.WriteString("  ")
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	return result.String()
}
