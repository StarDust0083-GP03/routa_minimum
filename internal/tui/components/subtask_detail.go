// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"
	"strings"

	"codeg/internal/task"

	"github.com/charmbracelet/lipgloss"
)

// SubTaskDetail shows detailed information about a selected sub-task.
type SubTaskDetail struct {
	subTask *task.SubTask
	width   int
	height  int
}

// NewSubTaskDetail creates a new sub-task detail component.
func NewSubTaskDetail(width, height int) SubTaskDetail {
	return SubTaskDetail{width: width, height: height}
}

// SetSubTask sets the sub-task to display.
func (s *SubTaskDetail) SetSubTask(st *task.SubTask) {
	s.subTask = st
}

// Resize updates dimensions.
func (s *SubTaskDetail) Resize(width, height int) {
	s.width = width
	s.height = height
}

// View renders the sub-task detail panel.
func (s SubTaskDetail) View() string {
	if s.subTask == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585B70")).
			Render("  No sub-task selected.")
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#89B4FA")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#89B4FA"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CDD6F4"))

	dirStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6E3A1"))

	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#585B70"))

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render("SubTask Detail") + "\n\n")

	// Fields
	sb.WriteString(labelStyle.Render("Title:") + " " + valueStyle.Render(s.subTask.Title) + "\n")
	sb.WriteString(labelStyle.Render("Dir:") + " " + dirStyle.Render(s.subTask.Directory) + "\n")
	sb.WriteString(labelStyle.Render("Status:") + " " + statusColored(s.subTask.Status) + "\n\n")

	// Description
	if s.subTask.Description != "" {
		sb.WriteString(mutedStyle.Render("Description:") + "\n")
		sb.WriteString(valueStyle.Render("  "+s.subTask.Description) + "\n\n")
	}

	// Phase progress
	sb.WriteString(s.renderPhaseProgress() + "\n\n")

	// Session info
	if s.subTask.CodingSessionID != "" {
		sb.WriteString(mutedStyle.Render(fmt.Sprintf("Coding session: %s\n", s.subTask.CodingSessionID)))
	}
	if s.subTask.VerificationSessionID != "" {
		sb.WriteString(mutedStyle.Render(fmt.Sprintf("Verification: %s\n", s.subTask.VerificationSessionID)))
	}

	// Timestamps
	if s.subTask.CodingStartedAt != nil {
		sb.WriteString(mutedStyle.Render(fmt.Sprintf("Coding started: %s\n", s.subTask.CodingStartedAt.Format("15:04:05"))))
	}
	if s.subTask.VerificationCompletedAt != nil {
		sb.WriteString(mutedStyle.Render(fmt.Sprintf("Verify done: %s\n", s.subTask.VerificationCompletedAt.Format("15:04:05"))))
	}

	return sb.String()
}

func (s SubTaskDetail) renderPhaseProgress() string {
	codingDot := "○"
	verifyDot := "○"
	doneDot := "○"

	codingColor := "#585B70"
	verifyColor := "#585B70"
	doneColor := "#585B70"

	switch s.subTask.Status {
	case task.SubTaskCoding:
		codingDot = "●"
		codingColor = "#F9E2AF"
	case task.SubTaskVerifying:
		codingDot = "●"
		codingColor = "#A6E3A1"
		verifyDot = "●"
		verifyColor = "#94E2D5"
	case task.SubTaskDone:
		codingDot = "●"
		codingColor = "#A6E3A1"
		verifyDot = "●"
		verifyColor = "#A6E3A1"
		doneDot = "●"
		doneColor = "#A6E3A1"
	case task.SubTaskStuck:
		codingDot = "⚠"
		codingColor = "#FAB387"
	case task.SubTaskFailed:
		codingDot = "●"
		codingColor = "#F38BA8"
	}

	return fmt.Sprintf("%s Coding ──── %s Verifying ──── %s Done",
		lipgloss.NewStyle().Foreground(lipgloss.Color(codingColor)).Render(codingDot),
		lipgloss.NewStyle().Foreground(lipgloss.Color(verifyColor)).Render(verifyDot),
		lipgloss.NewStyle().Foreground(lipgloss.Color(doneColor)).Render(doneDot),
	)
}

func statusColored(status task.SubTaskStatus) string {
	switch status {
	case task.SubTaskPlanned:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render(string(status))
	case task.SubTaskCoding:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render(string(status))
	case task.SubTaskVerifying:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#94E2D5")).Render(string(status))
	case task.SubTaskStuck:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FAB387")).Render(string(status))
	case task.SubTaskDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render(string(status))
	case task.SubTaskFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(string(status))
	}
	return string(status)
}
