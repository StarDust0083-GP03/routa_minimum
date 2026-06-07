// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"github.com/charmbracelet/lipgloss"
)

// Dashboard layout utilities.

var (
	dashboardDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#313244"))
)

// DashboardLayout renders the main three-panel layout.
// Returns the full terminal view as a string.
func DashboardLayout(
	taskListView string,
	detailView string,
	agentView string,
	statusBarView string,
	helpBarView string,
	terminalWidth int,
	terminalHeight int,
	taskListWidth int,
	focusedPanel int, // 1=tasks, 2=detail, 3=agent
) string {
	// Account for status bar (1 line) and help bar (1 line)
	mainHeight := terminalHeight - 2

	// Top half: task list + detail
	topHeight := mainHeight * 3 / 5
	bottomHeight := mainHeight - topHeight

	rightWidth := terminalWidth - taskListWidth

	// Style panels based on focus
	taskStyle := lipgloss.NewStyle().
		Width(taskListWidth).
		Height(topHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A"))

	detailStyle := lipgloss.NewStyle().
		Width(rightWidth).
		Height(topHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A"))

	bottomStyle := lipgloss.NewStyle().
		Width(terminalWidth).
		Height(bottomHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A"))

	switch focusedPanel {
	case 1:
		taskStyle = taskStyle.BorderForeground(lipgloss.Color("#89B4FA"))
	case 2:
		detailStyle = detailStyle.BorderForeground(lipgloss.Color("#89B4FA"))
	case 3:
		bottomStyle = bottomStyle.BorderForeground(lipgloss.Color("#89B4FA"))
	}

	// Render top row: task list + detail side by side
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		taskStyle.Render(taskListView),
		detailStyle.Render(detailView),
	)

	// Render bottom row
	bottomRow := bottomStyle.Render(agentView)

	// Combine everything
	mainArea := lipgloss.JoinVertical(
		lipgloss.Left,
		topRow,
		bottomRow,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusBarView,
		mainArea,
		helpBarView,
	)
}
