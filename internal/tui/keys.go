// Package tui provides the Bubble Tea terminal UI for the coding agent manager.
package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings grouped by mode.
type KeyMap struct {
	// Global keys (work in any mode)
	Quit      key.Binding
	Back      key.Binding
	ForceQuit key.Binding

	// Dashboard keys
	Up        key.Binding
	Down      key.Binding
	NewTask   key.Binding
	EditTask  key.Binding
	RunAgent  key.Binding
	Cancel    key.Binding
	Delete    key.Binding
	BindDir   key.Binding
	ViewTree  key.Binding
	Focus1    key.Binding
	Focus2    key.Binding
	Focus3    key.Binding
	Enter     key.Binding

	// Form keys
	NextField key.Binding
	PrevField key.Binding
	Submit    key.Binding
}

// DashboardKeys returns dashboard-mode keybindings.
func DashboardKeys() KeyMap {
	return KeyMap{
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),

		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		NewTask:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task")),
		EditTask: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		RunAgent: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run")),
		Cancel:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel")),
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		BindDir:  key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "bind dir")),
		ViewTree: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sessions")),
		Focus1:   key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "tasks")),
		Focus2:   key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "detail")),
		Focus3:   key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "agent")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),

		NextField: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		PrevField: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		Submit:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
	}
}
