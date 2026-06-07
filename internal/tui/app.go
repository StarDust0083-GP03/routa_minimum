// Package tui provides the Bubble Tea terminal UI for the coding agent manager.
package tui

import (
	"context"
	"fmt"

	"codeg/internal/agent/core"
	"codeg/internal/task"
	"codeg/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mode represents the current UI mode.
type Mode string

const (
	ModeDashboard  Mode = "dashboard"
	ModeCreateTask Mode = "create"
	ModeConfirm    Mode = "confirm"
	ModeBindDir    Mode = "bind_dir"
)

// --- Custom messages ---

type tasksLoadedMsg []*task.Task

type agentStartedMsg struct {
	taskID    string
	taskName  string
	sessionID string
}

type agentErrorMsg struct {
	taskID string
	err    error
}

// --- Model ---

// Model is the root Bubble Tea model.
type Model struct {
	// Services
	taskManager     *task.TaskManager
	agentController *core.AgentController
	taskStore       task.TaskStore

	// Agent event channel — goroutines write here, TUI reads via tea.Cmd
	agentEvents chan tea.Msg

	// State
	tasks         []*task.Task
	selectedIndex int
	mode          Mode
	focusedPanel  int // 1=tasks, 2=detail, 3=agent
	errorMsg      string
	activeTaskID  string

	// Components
	taskList   components.TaskList
	taskDetail components.TaskDetail
	agentPanel components.AgentPanel
	createForm components.CreateTaskForm

	// Bind directory input
	bindInput string

	// Dimensions
	width  int
	height int
	ready  bool
}

// NewModel creates a new TUI model.
func NewModel(
	taskMgr *task.TaskManager,
	agentCtrl *core.AgentController,
	taskStore task.TaskStore,
) Model {
	return Model{
		taskManager:     taskMgr,
		agentController: agentCtrl,
		taskStore:       taskStore,
		mode:            ModeDashboard,
		focusedPanel:    1,
		selectedIndex:   0,
		agentEvents:     make(chan tea.Msg, 256),
	}
}

// --- Init ---

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadTasks(),
		tea.SetWindowTitle("codeg"),
		m.waitForAgentEvent(),
	)
}

func (m *Model) waitForAgentEvent() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.agentEvents
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) loadTasks() tea.Cmd {
	return func() tea.Msg {
		tasks, err := m.taskManager.ListAll()
		if err != nil {
			return err
		}
		return tasksLoadedMsg(tasks)
	}
}

// --- Commands ---

func (m *Model) createTaskCmd(title, objective string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := m.taskManager.Create(ctx, title, objective)
		if err != nil {
			return err
		}
		tasks, err := m.taskManager.ListAll()
		if err != nil {
			return err
		}
		return tasksLoadedMsg(tasks)
	}
}

func (m *Model) runAgentCmd(t *task.Task) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background() // ACP client has its own 10s RPC timeout

		if len(t.BoundDirs) == 0 {
			return agentErrorMsg{t.ID, fmt.Errorf(
				"no directories bound to task '%s'. Press 'b' to bind one.", t.Title,
			)}
		}

		// Move task to in_progress
		_ = m.taskManager.StartTask(ctx, t.ID)

		prompt := fmt.Sprintf("You are working on task: %s\nObjective: %s",
			t.Title, t.Objective)

		handle, err := m.agentController.StartAgent(
			ctx, t, t.BoundDirs[0].Path, prompt,
		)
		if err != nil {
			return agentErrorMsg{t.ID, fmt.Errorf("failed to start agent: %w", err)}
		}

		// Stream events to the TUI via the agentEvents channel.
		go func() {
			for evt := range handle.Events {
				text := ""
				eventType := evt.Type

				switch evt.Type {
				case "agent_message_chunk":
					if v, ok := evt.Data["text"].(string); ok {
						text = v
					}
				case "tool_call":
					if name, ok := evt.Data["name"].(string); ok {
						text = name
					}
				case "tool_call_update":
					if out, ok := evt.Data["output"].(string); ok {
						text = out
					}
				case "process_output":
					if out, ok := evt.Data["text"].(string); ok {
						text = out
					}
				case "turn_complete":
					text = "--- turn complete ---"
				case "error":
					if errMsg, ok := evt.Data["message"].(string); ok {
						text = errMsg
					}
				}

				m.agentEvents <- components.AgentEventMsg{
					TaskID:    t.ID,
					EventType: eventType,
					Text:      text,
				}
			}
			// Agent finished
			m.agentEvents <- components.AgentDoneMsg{TaskID: t.ID}
		}()

		return agentStartedMsg{taskID: t.ID, taskName: t.Title, sessionID: handle.SessionID}
	}
}

// linkSessionCmd persists the agent session ID to the task.
func (m *Model) linkSessionCmd(taskID, sessionID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.taskManager.LinkSession(ctx, taskID, sessionID); err != nil {
			return err
		}
		return nil
	}
}

// completeTaskCmd completes the task and triggers summary generation via OpenAI.
func (m *Model) completeTaskCmd(taskID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.taskManager.CompleteTask(ctx, taskID); err != nil {
			return fmt.Errorf("failed to complete task: %w", err)
		}
		tasks, err := m.taskManager.ListAll()
		if err != nil {
			return err
		}
		return tasksLoadedMsg(tasks)
	}
}

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.ready = true
			m.initComponents()
		}
		m.resizeComponents()
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case ModeCreateTask:
			return m.handleCreateFormKey(msg)
		case ModeConfirm:
			return m.handleConfirmKey(msg)
		case ModeBindDir:
			return m.handleBindDirKey(msg)
		default:
			return m.handleDashboardKey(msg, &cmds)
		}

	case tasksLoadedMsg:
		m.tasks = msg
		m.taskList.SetTasks(m.tasks)
		if m.selectedIndex < len(m.tasks) && m.selectedIndex >= 0 {
			m.taskDetail.SetTask(m.tasks[m.selectedIndex])
		}
		return m, nil

	case components.TaskListMsg:
		m.selectedIndex = msg.Index
		m.taskDetail.SetTask(msg.Task)
		return m, nil

	case components.CreateTaskDoneMsg:
		if !msg.Cancelled && msg.Title != "" {
			cmds = append(cmds, m.createTaskCmd(msg.Title, msg.Objective))
		}
		m.mode = ModeDashboard
		m.errorMsg = ""
		return m, tea.Batch(cmds...)

	case agentStartedMsg:
		m.activeTaskID = msg.taskID
		m.agentPanel.SetActive(msg.taskID, msg.taskName)
		m.errorMsg = ""
		// Link session to task and refresh
		if msg.sessionID != "" {
			cmds = append(cmds, m.linkSessionCmd(msg.taskID, msg.sessionID))
		}
		cmds = append(cmds, m.loadTasks())
		return m, tea.Batch(cmds...)

	case agentErrorMsg:
		m.errorMsg = msg.err.Error()
		return m, nil

	case components.AgentEventMsg:
		var cmd tea.Cmd
		m.agentPanel, cmd = m.agentPanel.Update(msg)
		cmds = append(cmds, cmd, m.waitForAgentEvent())
		return m, tea.Batch(cmds...)

	case components.AgentDoneMsg:
		if msg.TaskID == m.activeTaskID {
			m.activeTaskID = ""
			m.agentController.RemoveHandle(msg.TaskID)
			m.agentPanel.SetInactive()
			// Auto-complete task and generate summary
			cmds = append(cmds, m.completeTaskCmd(msg.TaskID))
		}
		var cmd tea.Cmd
		m.agentPanel, cmd = m.agentPanel.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case error:
		m.errorMsg = msg.Error()
		return m, nil
	}

	if m.mode == ModeCreateTask {
		var cmd tea.Cmd
		m.createForm, cmd = m.createForm.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// --- Dashboard key handler ---

func (m *Model) handleDashboardKey(msg tea.KeyMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "ctrl+c":
		return m, tea.Quit

	case "up", "k", "down", "j":
		var cmd tea.Cmd
		m.taskList, cmd = m.taskList.Update(msg)
		*cmds = append(*cmds, cmd)
		if t := m.taskList.SelectedTask(); t != nil {
			m.taskDetail.SetTask(t)
		}

	case "1":
		m.focusedPanel = 1
	case "2":
		m.focusedPanel = 2
	case "3":
		m.focusedPanel = 3

	case "n":
		m.mode = ModeCreateTask
		m.createForm = components.NewCreateTaskForm()
		*cmds = append(*cmds, m.createForm.Init())

	case "r":
		if t := m.taskList.SelectedTask(); t != nil {
			if m.activeTaskID != "" {
				m.errorMsg = fmt.Sprintf("Agent already running (task: %s). Press 'c' to cancel first.", m.activeTaskID)
			} else {
				*cmds = append(*cmds, m.runAgentCmd(t))
			}
		}

	case "c":
		if m.activeTaskID == "" {
			m.errorMsg = "No agent running."
		} else {
			if err := m.agentController.CancelAgent(m.activeTaskID); err != nil {
				m.errorMsg = err.Error()
			} else {
				m.activeTaskID = ""
				m.agentPanel.SetInactive()
				m.errorMsg = "Agent cancelled."
			}
		}

	case "d":
		if t := m.taskList.SelectedTask(); t != nil {
			m.mode = ModeConfirm
			m.errorMsg = fmt.Sprintf("Delete task '%s'? (y/n)", t.Title)
		}

	case "b":
		if t := m.taskList.SelectedTask(); t != nil {
			m.mode = ModeBindDir
			m.bindInput = ""
			m.errorMsg = "Enter directory path (esc to cancel):"
		}

	case "f":
		if t := m.taskList.SelectedTask(); t != nil {
			if t.Status == "completed" || t.Status == "cancelled" {
				m.errorMsg = "Task already " + string(t.Status)
			} else {
				*cmds = append(*cmds, m.completeTaskCmd(t.ID))
				m.errorMsg = "Completing task..."
			}
		}

	case "e":
		if t := m.taskList.SelectedTask(); t != nil {
			m.errorMsg = "Edit: use 'b' to bind dirs, 'r' to run agent. Task: " + t.Title
		}

	case "enter":
		m.focusedPanel = 2
	}

	return m, tea.Batch(*cmds...)
}

// --- Create form handler ---

func (m *Model) handleCreateFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeDashboard
		m.errorMsg = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.createForm, cmd = m.createForm.Update(msg)
	return m, cmd
}

// --- Confirm handler ---

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		t := m.taskList.SelectedTask()
		if t != nil {
			_ = m.taskManager.Delete(t.ID)
			m.mode = ModeDashboard
			m.errorMsg = ""
			return m, m.loadTasks()
		}
		m.mode = ModeDashboard
		return m, nil
	case "n", "N", "esc":
		m.mode = ModeDashboard
		m.errorMsg = ""
		return m, nil
	}
	return m, nil
}

// --- Bind dir handler ---

func (m *Model) handleBindDirKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeDashboard
		m.errorMsg = ""
		return m, nil
	case "enter":
		if m.bindInput != "" {
			t := m.taskList.SelectedTask()
			if t != nil {
				ctx := context.Background()
				if err := m.taskManager.BindDirectory(ctx, t.ID, m.bindInput, ""); err != nil {
					m.errorMsg = err.Error()
				} else {
					m.mode = ModeDashboard
					m.errorMsg = ""
					return m, m.loadTasks()
				}
			}
		}
		m.mode = ModeDashboard
		return m, nil
	case "backspace":
		if len(m.bindInput) > 0 {
			m.bindInput = m.bindInput[:len(m.bindInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.bindInput += msg.String()
		}
	}
	return m, nil
}

// --- View ---

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.mode == ModeCreateTask {
		return m.renderCreateForm()
	}

	statusBar := components.StatusBar(
		m.width,
		len(m.tasks),
		len(m.agentController.RunningTasks()),
		string(m.mode),
		m.errorMsg,
	)

	helpBar := components.HelpBar(m.width, components.DashboardHelp())

	taskListView := m.taskList.View()
	detailView := m.taskDetail.View()
	agentView := m.agentPanel.View()

	taskListWidth := m.width / 3

	mainView := components.DashboardLayout(
		taskListView,
		detailView,
		agentView,
		statusBar,
		helpBar,
		m.width,
		m.height,
		taskListWidth,
		m.focusedPanel,
	)

	if m.mode == ModeConfirm {
		mainView = m.overlayConfirm()
	}
	if m.mode == ModeBindDir {
		mainView = m.overlayBindDir()
	}

	return mainView
}

func (m *Model) renderCreateForm() string {
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		m.createForm.View(),
	)
}

func (m *Model) overlayConfirm() string {
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F9E2AF")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(1, 3).
		Render(m.errorMsg)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#11111B")),
	)
}

func (m *Model) overlayBindDir() string {
	inputDisplay := m.bindInput
	if inputDisplay == "" {
		inputDisplay = "_"
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#89B4FA")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(1, 3).
		Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA")).Render("Bind Directory") + "\n\n" +
				m.errorMsg + "\n" +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("#A6E3A1")).
					Render("> "+inputDisplay) + "\n\n" +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("#585B70")).
					Render("enter: confirm  esc: cancel"),
		)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#11111B")),
	)
}

// --- Layout helpers ---

func (m *Model) initComponents() {
	m.taskList = components.NewTaskList(m.width/3, (m.height-2)*3/5)
	m.taskDetail = components.NewTaskDetail(m.width-m.width/3, (m.height-2)*3/5)
	m.agentPanel = components.NewAgentPanel(m.width, (m.height-2)*2/5)
	m.createForm = components.NewCreateTaskForm()
}

func (m *Model) resizeComponents() {
	taskListWidth := m.width / 3
	rightWidth := m.width - taskListWidth
	topHeight := (m.height - 2) * 3 / 5
	bottomHeight := (m.height - 2) * 2 / 5

	m.taskList.Resize(taskListWidth-4, topHeight-2)
	m.taskDetail.Resize(rightWidth-4, topHeight-2)
	m.agentPanel.Resize(m.width-4, bottomHeight-2)
}

func (m *Model) GetTaskManager() *task.TaskManager { return m.taskManager }
func (m *Model) GetAgentController() *core.AgentController { return m.agentController }
