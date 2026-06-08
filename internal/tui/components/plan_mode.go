// Package components provides UI components for the TUI coding agent manager.
package components

import (
	"fmt"
	"strings"

	"codeg/internal/task"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlanModeState tracks the plan mode workflow.
type PlanModeState int

const (
	PlanStateLoading PlanModeState = iota
	PlanStateEditing
	PlanStateConfirming
)

// PlanModeMsg is sent when the plan mode is done.
type PlanModeMsg struct {
	Cancelled bool
	Plans     []task.SubTaskPlan
	TaskID    string
}

// PlanMode manages the task decomposition UI.
type PlanMode struct {
	state    PlanModeState
	taskID   string
	taskTitle string

	plans       []task.SubTaskPlan
	selectedIdx int
	editingIdx  int // -1 = not editing

	// Editing inputs
	titleInput   textinput.Model
	descInput    textinput.Model
	dirInput     textinput.Model

	width  int
	height int
	ready  bool
}

// NewPlanMode creates a new plan mode component.
func NewPlanMode(taskID, taskTitle string) PlanMode {
	ti := textinput.New()
	ti.Placeholder = "Sub-task title"
	ti.CharLimit = 80

	di := textinput.New()
	di.Placeholder = "Description"
	di.CharLimit = 256

	diri := textinput.New()
	diri.Placeholder = "Directory path"
	diri.CharLimit = 256

	return PlanMode{
		state:      PlanStateLoading,
		taskID:     taskID,
		taskTitle:  taskTitle,
		selectedIdx: 0,
		editingIdx:  -1,
		titleInput: ti,
		descInput:  di,
		dirInput:   diri,
	}
}

// SetPlans sets the plans received from the LLM.
func (p *PlanMode) SetPlans(plans []task.SubTaskPlan) {
	p.plans = plans
	p.state = PlanStateEditing
}

// SetError sets an error state.
func (p *PlanMode) SetError(errMsg string) {
	p.plans = []task.SubTaskPlan{{
		Title:       "Error",
		Description: errMsg,
		Directory:   "",
		OrderIndex:  0,
	}}
	p.state = PlanStateEditing
}

// Init initializes the component.
func (p PlanMode) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (p PlanMode) Update(msg tea.Msg) (PlanMode, tea.Cmd) {
	if p.editingIdx >= 0 {
		return p.updateEditing(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg {
				return PlanModeMsg{Cancelled: true, TaskID: p.taskID}
			}

		case "enter":
			if len(p.plans) > 0 && p.plans[0].Title != "Error" {
				return p, func() tea.Msg {
					return PlanModeMsg{
						Cancelled: false,
						Plans:     p.plans,
						TaskID:    p.taskID,
					}
				}
			}

		case "up", "k":
			if p.selectedIdx > 0 {
				p.selectedIdx--
			}

		case "down", "j":
			if p.selectedIdx < len(p.plans)-1 {
				p.selectedIdx++
			}

		case "e":
			if p.selectedIdx >= 0 && p.selectedIdx < len(p.plans) {
				p.editingIdx = p.selectedIdx
				plan := p.plans[p.selectedIdx]
				p.titleInput.SetValue(plan.Title)
				p.descInput.SetValue(plan.Description)
				p.dirInput.SetValue(plan.Directory)
				p.titleInput.Focus()
				return p, textinput.Blink
			}

		case "+":
			p.plans = append(p.plans, task.SubTaskPlan{
				Title:       "New sub-task",
				Description: "Describe what to implement",
				Directory:   "",
				OrderIndex:  len(p.plans) + 1,
			})
			p.selectedIdx = len(p.plans) - 1

		case "-":
			if len(p.plans) > 0 && p.selectedIdx < len(p.plans) {
				p.plans = append(p.plans[:p.selectedIdx], p.plans[p.selectedIdx+1:]...)
				if p.selectedIdx >= len(p.plans) {
					p.selectedIdx = len(p.plans) - 1
				}
			}
		}
	}

	return p, nil
}

func (p PlanMode) updateEditing(msg tea.Msg) (PlanMode, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			p.editingIdx = -1
			p.titleInput.Blur()
			p.descInput.Blur()
			p.dirInput.Blur()
			return p, nil

		case "enter":
			// Save edits
			plan := &p.plans[p.editingIdx]
			plan.Title = p.titleInput.Value()
			plan.Description = p.descInput.Value()
			plan.Directory = p.dirInput.Value()
			plan.OrderIndex = p.editingIdx + 1
			p.editingIdx = -1
			p.titleInput.Blur()
			p.descInput.Blur()
			p.dirInput.Blur()
			return p, nil

		case "tab":
			if p.titleInput.Focused() {
				p.titleInput.Blur()
				p.descInput.Focus()
			} else if p.descInput.Focused() {
				p.descInput.Blur()
				p.dirInput.Focus()
			} else {
				p.dirInput.Blur()
				p.titleInput.Focus()
			}
		}
	}

	var cmd tea.Cmd
	if p.titleInput.Focused() {
		p.titleInput, cmd = p.titleInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if p.descInput.Focused() {
		p.descInput, cmd = p.descInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if p.dirInput.Focused() {
		p.dirInput, cmd = p.dirInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

// View renders the plan mode UI.
func (p PlanMode) View() string {
	if p.state == PlanStateLoading {
		return p.renderLoading()
	}
	return p.renderEditing()
}

func (p PlanMode) renderLoading() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#CBA6F7")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(1, 3).
		Width(70).
		Render(
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBA6F7")).
				Bold(true).
				Render("Plan Mode") + "\n\n" +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("#A6ADC8")).
					Render("Decomposing task with LLM...") + "\n\n" +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("#89DCEB")).
					Render("Task: "+p.taskTitle),
		)
}

func (p PlanMode) renderEditing() string {
	var sb strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CBA6F7")).
		Bold(true).
		Render("Plan Mode") + "  " +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6ADC8")).
			Render("Task: "+p.taskTitle)

	sb.WriteString(header + "\n\n")

	// Instructions
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#585B70")).
		Render("LLM suggests the following sub-tasks. Edit or confirm.") + "\n\n")

	// Sub-task list
	for i, plan := range p.plans {
		prefix := " "
		if i == p.selectedIdx {
			prefix = "▶"
		}

		borderColor := "#45475A"
		if i == p.selectedIdx {
			borderColor = "#89B4FA"
		}

		entry := fmt.Sprintf("%s \033[38;2;137;180;250m%d.\033[0m \033[38;2;205;214;244m%s\033[0m",
			prefix, i+1, plan.Title)

		entry += "\n   \033[38;2;166;173;200m" + plan.Description + "\033[0m"
		entry += "\n   \033[38;2;148;226;213m" + plan.Directory + "\033[0m"

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(borderColor)).
			Background(lipgloss.Color("#313244")).
			Padding(0, 1).
			Width(68).
			Render(entry)

		sb.WriteString(box + "\n")
	}

	// Editing overlay
	if p.editingIdx >= 0 {
		sb.WriteString("\n")
		sb.WriteString(p.renderEditForm())
	}

	// Help bar
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#585B70")).
		Render("Enter: confirm  e: edit  +: add  -: remove  ↑/↓: navigate  Esc: cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#CBA6F7")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(1, 2).
		Width(72).
		Render(sb.String())
}

func (p PlanMode) renderEditForm() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F9E2AF")).
		Background(lipgloss.Color("#313244")).
		Padding(1, 1).
		Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Bold(true).Render("Editing sub-task") + "\n" +
				"Title:      "+p.titleInput.View()+"\n" +
				"Description:"+p.descInput.View()+"\n" +
				"Directory:  "+p.dirInput.View()+"\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("Enter: save  Tab: next  Esc: cancel"),
		)
}
