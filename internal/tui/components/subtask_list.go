// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"
	"strings"

	"codeg/internal/task"

	"github.com/charmbracelet/lipgloss"
)

// SubTaskList renders a list of sub-tasks under a task.
type SubTaskList struct {
	subTasks    []*task.SubTask
	selectedIdx int
	width       int
	height      int
}

// NewSubTaskList creates a new sub-task list component.
func NewSubTaskList(width, height int) SubTaskList {
	return SubTaskList{
		width:       width,
		height:      height,
		selectedIdx: 0,
	}
}

// SetSubTasks updates the sub-task list.
func (s *SubTaskList) SetSubTasks(subTasks []*task.SubTask) {
	s.subTasks = subTasks
	if s.selectedIdx >= len(s.subTasks) && len(s.subTasks) > 0 {
		s.selectedIdx = len(s.subTasks) - 1
	}
}

// SelectedSubTask returns the currently selected sub-task.
func (s *SubTaskList) SelectedSubTask() *task.SubTask {
	if s.selectedIdx >= 0 && s.selectedIdx < len(s.subTasks) {
		return s.subTasks[s.selectedIdx]
	}
	return nil
}

// SelectedIndex returns the selected index.
func (s *SubTaskList) SelectedIndex() int {
	return s.selectedIdx
}

// MoveUp moves selection up.
func (s *SubTaskList) MoveUp() {
	if s.selectedIdx > 0 {
		s.selectedIdx--
	}
}

// MoveDown moves selection down.
func (s *SubTaskList) MoveDown() {
	if s.selectedIdx < len(s.subTasks)-1 {
		s.selectedIdx++
	}
}

// Resize updates dimensions.
func (s *SubTaskList) Resize(width, height int) {
	s.width = width
	s.height = height
}

// View renders the sub-task list.
func (s SubTaskList) View() string {
	if len(s.subTasks) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585B70")).
			Render("  No sub-tasks. Press 'p' to plan.")
	}

	var sb strings.Builder

	for i, st := range s.subTasks {
		// Selection indicator
		prefix := "  "
		if i == s.selectedIdx {
			prefix = "▶ "
		}

		// Status dot
		dot := s.statusDot(st.Status)
		titleColor := "#CDD6F4"
		if i == s.selectedIdx {
			titleColor = "#A6E3A1"
		}

		line := fmt.Sprintf("%s%s \033[38;2;%sm%s\033[0m",
			prefix, dot, hexToRGB(titleColor), st.Title)

		sb.WriteString(line + "\n")

		// Indented directory
		indent := "   "
		if i == s.selectedIdx {
			indent = "  "
		}
		sb.WriteString(fmt.Sprintf("%s\033[38;2;166;173;200m%s\033[0m\n",
			indent+"  ", st.Directory))

		// Phase status
		phaseText := s.phaseText(st)
		sb.WriteString(fmt.Sprintf("%s%s\n", indent+"  ", phaseText))
	}

	return sb.String()
}

func (s SubTaskList) statusDot(status task.SubTaskStatus) string {
	switch status {
	case task.SubTaskPlanned:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("○")
	case task.SubTaskCoding:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("◉")
	case task.SubTaskVerifying:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#94E2D5")).Render("◷")
	case task.SubTaskDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("✓")
	case task.SubTaskStuck:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FAB387")).Render("⚠")
	case task.SubTaskFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("✗")
	}
	return " "
}

func (s SubTaskList) phaseText(st *task.SubTask) string {
	switch st.Status {
	case task.SubTaskPlanned:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("planned")
	case task.SubTaskCoding:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render("coding...")
	case task.SubTaskVerifying:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#94E2D5")).Render("verifying...")
	case task.SubTaskStuck:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FAB387")).Render("⚠ stuck")
	case task.SubTaskDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("✓ done")
	case task.SubTaskFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("✗ failed")
	}
	return ""
}

// hexToRGB converts a hex color to RGB format for ANSI escape codes.
func hexToRGB(hex string) string {
	// Strip # prefix
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "205;214;244"
	}
	r := hexToInt(hex[0:2])
	g := hexToInt(hex[2:4])
	b := hexToInt(hex[4:6])
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

func hexToInt(hex string) int {
	var v int
	fmt.Sscanf(hex, "%x", &v)
	return v
}
