// Package tui provides the Bubble Tea terminal UI for the coding agent manager.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	ColorGreen   = lipgloss.Color("#4CAF50")
	ColorYellow  = lipgloss.Color("#FFC107")
	ColorRed     = lipgloss.Color("#F44336")
	ColorBlue    = lipgloss.Color("#2196F3")
	ColorGray    = lipgloss.Color("#9E9E9E")
	ColorCyan    = lipgloss.Color("#00BCD4")
	ColorMagenta = lipgloss.Color("#E91E63")
	ColorWhite   = lipgloss.Color("#FFFFFF")
	ColorBlack   = lipgloss.Color("#000000")
	ColorBG      = lipgloss.Color("#1E1E2E")
	ColorBGLight = lipgloss.Color("#2E2E3E")
)

// Base styles.
var (
	StyleBase = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CDD6F4"))

	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorGray)

	StyleBold = lipgloss.NewStyle().
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorGreen)
)

// Panel styles.
var (
	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray)

	StyleFocusedPanel = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBlue)

	StylePanelTitle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)
)

// Status-specific styles.
var (
	StyleStatusPending = lipgloss.NewStyle().
				Foreground(ColorGray)

	StyleStatusInProgress = lipgloss.NewStyle().
				Foreground(ColorYellow)

	StyleStatusReview = lipgloss.NewStyle().
				Foreground(ColorCyan)

	StyleStatusCompleted = lipgloss.NewStyle().
				Foreground(ColorGreen)

	StyleStatusBlocked = lipgloss.NewStyle().
				Foreground(ColorRed)

	StyleStatusCancelled = lipgloss.NewStyle().
				Foreground(ColorRed).
				Strikethrough(true)
)

// StyleStatus returns the style for a given task status.
func StyleStatus(status string) lipgloss.Style {
	switch status {
	case "pending":
		return StyleStatusPending
	case "in_progress":
		return StyleStatusInProgress
	case "review":
		return StyleStatusReview
	case "completed":
		return StyleStatusCompleted
	case "blocked":
		return StyleStatusBlocked
	case "cancelled":
		return StyleStatusCancelled
	default:
		return StyleBase
	}
}

// StatusDot returns a colored dot for the given status.
func StatusDot(status string) string {
	switch status {
	case "pending":
		return StyleStatusPending.Render("●")
	case "in_progress":
		return StyleStatusInProgress.Render("●")
	case "review":
		return StyleStatusReview.Render("●")
	case "completed":
		return StyleStatusCompleted.Render("●")
	case "blocked":
		return StyleStatusBlocked.Render("●")
	case "cancelled":
		return StyleStatusCancelled.Render("●")
	default:
		return StyleBase.Render("●")
	}
}

// StatusLabel returns a styled label for the given status.
func StatusLabel(status string) string {
	switch status {
	case "pending":
		return StyleStatusPending.Render("PENDING")
	case "in_progress":
		return StyleStatusInProgress.Render("IN PROGRESS")
	case "review":
		return StyleStatusReview.Render("REVIEW")
	case "completed":
		return StyleStatusCompleted.Render("COMPLETED")
	case "blocked":
		return StyleStatusBlocked.Render("BLOCKED")
	case "cancelled":
		return StyleStatusCancelled.Render("CANCELLED")
	default:
		return status
	}
}

// Width and height utilities.
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
