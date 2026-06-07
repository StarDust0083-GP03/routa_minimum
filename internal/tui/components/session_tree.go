// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"
	"strings"

	"codeg/internal/session"

	"github.com/charmbracelet/lipgloss"
)

var (
	sessionTreeStyle = lipgloss.NewStyle()

	sessionRootStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89B4FA")).
				Bold(true)

	sessionBranchStyle = lipgloss.NewStyle()

	sessionEntryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CDD6F4"))

	sessionUserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89B4FA"))

	sessionAssistantStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6E3A1"))

	sessionToolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAB387"))

	sessionCompactionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBA6F7"))
)

// SessionTree renders a hierarchical view of session entries.
// It takes the output of SessionManager.GetTree() and renders an indented tree.
func SessionTree(tree []*session.SessionTreeNode, maxWidth int) string {
	if len(tree) == 0 {
		return sessionEntryStyle.Render("  No sessions recorded.")
	}

	var sb strings.Builder
	sb.WriteString(sessionRootStyle.Render("Sessions"))
	sb.WriteString("\n\n")

	for i, node := range tree {
		isLast := i == len(tree)-1
		renderTreeNode(&sb, node, "", isLast, maxWidth)
	}

	return sb.String()
}

func renderTreeNode(sb *strings.Builder, node *session.SessionTreeNode, prefix string, isLast bool, maxWidth int) {
	var connector string
	if isLast {
		connector = "└── "
	} else {
		connector = "├── "
	}

	// Build the entry label
	label := entryLabel(node)
	line := prefix + connector + label
	sb.WriteString(truncate(line, maxWidth))
	sb.WriteString("\n")

	// Determine child prefix
	var childPrefix string
	if isLast {
		childPrefix = prefix + "    "
	} else {
		childPrefix = prefix + "│   "
	}

	for i, child := range node.Children {
		childLast := i == len(node.Children)-1
		renderTreeNode(sb, child, childPrefix, childLast, maxWidth)
	}
}

func entryLabel(node *session.SessionTreeNode) string {
	switch node.Entry.Type {
	case session.EntryTypeMessage:
		if node.Entry.Message != nil {
			roleStyle := sessionUserStyle
			switch node.Entry.Message.Role {
			case session.RoleUser:
				roleStyle = sessionUserStyle
			case session.RoleAssistant:
				roleStyle = sessionAssistantStyle
			case session.RoleTool:
				roleStyle = sessionToolStyle
			}
			preview := messagePreview(node.Entry.Message)
			return fmt.Sprintf("%s: %s",
				roleStyle.Render(string(node.Entry.Message.Role)),
				preview,
			)
		}
		return string(node.Entry.Type)

	case session.EntryTypeCompaction:
		if node.Entry.Summary != nil {
			return sessionCompactionStyle.Render(
				fmt.Sprintf("compaction: %s", truncate(*node.Entry.Summary, 60)),
			)
		}
		return "compaction"

	case session.EntryTypeBranchSummary:
		if node.Entry.Summary != nil {
			return sessionCompactionStyle.Render(
				fmt.Sprintf("branch: %s", truncate(*node.Entry.Summary, 60)),
			)
		}
		return "branch summary"

	case session.EntryTypeLabel:
		if node.Entry.Label != nil {
			return fmt.Sprintf("label: %s", *node.Entry.Label)
		}
		return "label"

	case session.EntryTypeThinkingLevelChange:
		if node.Entry.ThinkingLevel != nil {
			return fmt.Sprintf("thinking: %s", *node.Entry.ThinkingLevel)
		}
		return "thinking change"

	case session.EntryTypeModelChange:
		provider := "?"
		model := "?"
		if node.Entry.Provider != nil {
			provider = *node.Entry.Provider
		}
		if node.Entry.ModelID != nil {
			model = *node.Entry.ModelID
		}
		return fmt.Sprintf("model: %s/%s", provider, model)

	case session.EntryTypeCustom:
		if node.Entry.CustomType != nil {
			return fmt.Sprintf("custom: %s", *node.Entry.CustomType)
		}
		return "custom"

	case session.EntryTypeSessionInfo:
		if node.Entry.Name != nil {
			return fmt.Sprintf("session: %s", *node.Entry.Name)
		}
		return "session info"

	default:
		return string(node.Entry.Type)
	}
}

func messagePreview(msg *session.AgentMessage) string {
	for _, c := range msg.Content {
		if c.Type == "text" && c.Text != "" {
			return truncate(c.Text, 50)
		}
		if c.Type == "tool_use" {
			return fmt.Sprintf("[%s]", c.ToolName)
		}
	}
	return "(empty)"
}
