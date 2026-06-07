// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1E1E2E")).
			Background(lipgloss.Color("#89B4FA")).
			Padding(0, 1).
			Bold(true)

	statusBarErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1E1E2E")).
				Background(lipgloss.Color("#F38BA8")).
				Padding(0, 1).
				Bold(true)
)

// StatusBar renders the top status bar.
func StatusBar(width int, taskCount, runningCount int, mode, errorMsg string) string {
	left := fmt.Sprintf("codeg | %d tasks | %d running | %s",
		taskCount, runningCount, mode)

	right := "q: quit"

	padding := width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}
	padded := left + fmt.Sprintf("%*s", padding, "") + right

	if errorMsg != "" {
		return statusBarErrorStyle.Width(width).Render(errorMsg)
	}
	return statusBarStyle.Width(width).Render(padded)
}
