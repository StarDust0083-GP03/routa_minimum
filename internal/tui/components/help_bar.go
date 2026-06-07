// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	helpBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585B70")).
			Background(lipgloss.Color("#181825")).
			Padding(0, 1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89B4FA")).
			Bold(true)
)

// HelpBar renders the bottom keybinding help bar.
// The bindings arg is a list of (key, description) pairs.
func HelpBar(width int, bindings [][2]string) string {
	var parts []string
	for _, b := range bindings {
		parts = append(parts, helpKeyStyle.Render(b[0])+" "+b[1])
	}
	text := strings.Join(parts, "  │  ")
	return helpBarStyle.Width(width).Render(text)
}

// DashboardHelp returns help bindings for dashboard mode.
func DashboardHelp() [][2]string {
	return [][2]string{
		{"q", "quit"},
		{"n", "new"},
		{"r", "run"},
		{"c", "cancel"},
		{"f", "finish"},
		{"b", "bind"},
		{"d", "delete"},
		{"1/2/3", "focus"},
		{"↑/↓", "nav"},
	}
}

// CreateFormHelp returns help bindings for the create task form.
func CreateFormHelp() [][2]string {
	return [][2]string{
		{"tab", "next"},
		{"enter", "submit"},
		{"esc", "cancel"},
	}
}
