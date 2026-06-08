// Package tui provides the Bubble Tea terminal UI for the coding agent manager.
package tui

import (
	"context"
	"fmt"

	"codeg/internal/acp"
	"codeg/internal/agent"
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
	ModePlan       Mode = "plan"
)

// --- Custom messages ---

type tasksLoadedMsg []*task.Task

type subTasksLoadedMsg struct {
	taskID   string
	subTasks []*task.SubTask
}

type plansLoadedMsg struct {
	taskID string
	plans  []task.SubTaskPlan
	err    error
}

type agentStartedMsg struct {
	taskID    string
	subTaskID string
	taskName  string
	sessionID string
	phase     string
}

type agentErrorMsg struct {
	taskID string
	err    error
}

type codingJudgeResultMsg struct {
	taskID    string
	subTaskID string
	verdict   task.CompletionVerdict
	reason    string
	taskObj   *task.Task
	st        *task.SubTask
}

type verifyJudgeResultMsg struct {
	taskID    string
	subTaskID string
	verdict   task.VerificationVerdict
	reason    string
	taskObj   *task.Task
}

// --- Model ---

type Model struct {
	taskManager     *task.TaskManager
	agentController *core.AgentController
	taskStore       task.TaskStore
	acpManager      *agent.AcpManager

	agentEvents chan tea.Msg

	tasks          []*task.Task
	subTasks       []*task.SubTask
	selectedIndex  int
	subSelectedIdx int
	mode           Mode
	focusedPanel   int // 1=tasks, 2=subtasks, 3=agent
	errorMsg       string
	activeTaskID   string
	activeSubID    string
	activePhase    string

	taskList    components.TaskList
	taskDetail  components.TaskDetail
	agentPanel  components.AgentPanel
	createForm  components.CreateTaskForm
	subTaskList components.SubTaskList
	subDetail   components.SubTaskDetail
	planMode    components.PlanMode

	bindInput string

	width  int
	height int
	ready  bool
}

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
		subSelectedIdx:  0,
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

func (m *Model) loadSubTasks(taskID string) tea.Cmd {
	return func() tea.Msg {
		if m.taskManager.SubTasks() == nil {
			return subTasksLoadedMsg{taskID: taskID}
		}
		subTasks, err := m.taskManager.SubTasks().ListByTask(taskID)
		if err != nil {
			return err
		}
		return subTasksLoadedMsg{taskID: taskID, subTasks: subTasks}
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

func (m *Model) planTaskCmd(taskObj *task.Task) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() == nil {
			return plansLoadedMsg{taskID: taskObj.ID, err: fmt.Errorf("sub-task manager not initialized")}
		}

		// Try ACP-based planning first
		plans, err := m.planViaACP(ctx, taskObj)
		if err == nil {
			return plansLoadedMsg{taskID: taskObj.ID, plans: plans}
		}

		// Fallback to OpenAI API if ACP fails or is unavailable
		llmClient := m.taskManager.GetLLMClient()
		if llmClient == nil {
			return plansLoadedMsg{taskID: taskObj.ID, err: fmt.Errorf("ACP planning failed and no LLM client configured: %w", err)}
		}
		planner := task.NewPlanner(llmClient, "")
		plans, llmErr := planner.DecomposeTask(ctx, taskObj)
		if llmErr != nil {
			return plansLoadedMsg{taskID: taskObj.ID, err: fmt.Errorf("ACP: %w; LLM fallback: %w", err, llmErr)}
		}
		return plansLoadedMsg{taskID: taskObj.ID, plans: plans}
	}
}

// planViaACP uses an ACP agent session to decompose a task into sub-tasks.
func (m *Model) planViaACP(ctx context.Context, taskObj *task.Task) ([]task.SubTaskPlan, error) {
	// Build the planning prompt
	systemPrompt, userMsg := task.BuildPlanningPrompt(taskObj)
	prompt := systemPrompt + "\n\n" + userMsg

	// Determine working directory
	cwd := "/tmp"
	if len(taskObj.BoundDirs) > 0 {
		cwd = taskObj.BoundDirs[0].Path
	}

	// Start a short-lived ACP session for planning
	result, err := m.agentController.PlanWithACP(ctx, cwd, prompt)
	if err != nil {
		return nil, fmt.Errorf("ACP plan session: %w", err)
	}

	// Parse the agent's JSON response
	plans, err := task.ParsePlans(result)
	if err != nil {
		return nil, fmt.Errorf("parse ACP plan response: %w", err)
	}

	// Assign order indices
	for i := range plans {
		plans[i].OrderIndex = i + 1
	}

	return plans, nil
}

func (m *Model) createSubTasksCmd(taskID string, plans []task.SubTaskPlan) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() == nil {
			return fmt.Errorf("sub-task manager not initialized")
		}
		_, err := m.taskManager.SubTasks().CreateFromPlans(ctx, taskID, plans)
		if err != nil {
			return err
		}
		_ = m.taskManager.StartTask(ctx, taskID)
		subTasks, _ := m.taskManager.SubTasks().ListByTask(taskID)
		return subTasksLoadedMsg{taskID: taskID, subTasks: subTasks}
	}
}

// agentEventText extracts display text from an ACP SSE event.
func agentEventText(evt acp.SSEEvent) string {
	switch evt.Type {
	case acp.EventMessageChunk:
		if v, ok := evt.Data["text"].(string); ok {
			return v
		}
	case acp.EventToolCall:
		if name, ok := evt.Data["name"].(string); ok {
			return name
		}
	case acp.EventToolUpdate:
		if out, ok := evt.Data["output"].(string); ok {
			return out
		}
	case acp.EventProcessOutput:
		if out, ok := evt.Data["text"].(string); ok {
			return out
		}
	case acp.EventSessionError:
		if errMsg, ok := evt.Data["message"].(string); ok {
			return errMsg
		}
	}
	return ""
}

func (m *Model) runSubTaskCodingCmd(st *task.SubTask, taskObj *task.Task) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() != nil {
			_, _ = m.taskManager.SubTasks().StartCoding(ctx, st.ID)
		}
		handle, err := m.agentController.StartSubTaskCoding(ctx, st, taskObj)
		if err != nil {
			return agentErrorMsg{taskObj.ID, fmt.Errorf("failed to start coding: %w", err)}
		}
		go m.streamAgentEvents(handle, taskObj, st, task.PhaseCoding)
		return agentStartedMsg{
			taskID: taskObj.ID, subTaskID: st.ID, taskName: st.Title,
			sessionID: handle.SessionID, phase: task.PhaseCoding,
		}
	}
}

func (m *Model) runSubTaskVerifyCmd(st *task.SubTask, taskObj *task.Task) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() != nil {
			_, _ = m.taskManager.SubTasks().StartVerifying(ctx, st.ID)
		}
		handle, err := m.agentController.StartSubTaskVerification(ctx, st, taskObj)
		if err != nil {
			return agentErrorMsg{taskObj.ID, fmt.Errorf("failed to start verification: %w", err)}
		}
		go m.streamAgentEvents(handle, taskObj, st, task.PhaseVerifying)
		return agentStartedMsg{
			taskID: taskObj.ID, subTaskID: st.ID, taskName: st.Title,
			sessionID: handle.SessionID, phase: task.PhaseVerifying,
		}
	}
}

// streamAgentEvents forwards ACP events from a handle to the TUI event channel.
func (m *Model) streamAgentEvents(handle *core.AgentHandle, taskObj *task.Task, st *task.SubTask, phase string) {
	for evt := range handle.Events {
		m.agentEvents <- components.AgentEventMsg{
			TaskID: taskObj.ID, SubTaskID: st.ID, Phase: phase,
			EventType: evt.Type, Text: agentEventText(evt),
		}
		if evt.Type == acp.EventTurnComplete {
			break
		}
	}
	m.agentEvents <- components.AgentDoneMsg{
		TaskID: taskObj.ID, SubTaskID: st.ID, Phase: phase,
	}
}

func (m *Model) linkSessionCmd(taskID, sessionID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.taskManager.LinkSession(ctx, taskID, sessionID); err != nil {
			return err
		}
		return nil
	}
}

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

func (m *Model) markSubTaskDoneCmd(subTaskID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() != nil {
			if _, err := m.taskManager.SubTasks().MarkDone(ctx, subTaskID); err != nil {
				return err
			}
		}
		return nil
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
		case ModePlan:
			return m.handlePlanModeKey(msg)
		default:
			return m.handleDashboardKey(msg, &cmds)
		}

	case tasksLoadedMsg:
		m.tasks = msg
		m.taskList.SetTasks(m.tasks)
		if m.selectedIndex >= len(m.tasks) {
			m.selectedIndex = max(0, len(m.tasks)-1)
		}
		if m.selectedIndex < len(m.tasks) && m.selectedIndex >= 0 {
			m.taskDetail.SetTask(m.tasks[m.selectedIndex])
			cmds = append(cmds, m.loadSubTasks(m.tasks[m.selectedIndex].ID))
		}
		return m, tea.Batch(cmds...)

	case subTasksLoadedMsg:
		if msg.taskID == m.activeTaskID || m.selectedTaskID() == msg.taskID {
			m.subTasks = msg.subTasks
			m.subTaskList.SetSubTasks(m.subTasks)
			if m.subSelectedIdx >= len(m.subTasks) {
				m.subSelectedIdx = max(0, len(m.subTasks)-1)
			}
			if st := m.subTaskList.SelectedSubTask(); st != nil {
				m.subDetail.SetSubTask(st)
			}
		}
		return m, nil

	case plansLoadedMsg:
		m.planMode.SetPlans(msg.plans)
		if msg.err != nil {
			m.planMode.SetError(msg.err.Error())
		}
		return m, nil

	case components.TaskListMsg:
		m.selectedIndex = msg.Index
		m.taskDetail.SetTask(msg.Task)
		m.subSelectedIdx = 0
		cmds = append(cmds, m.loadSubTasks(msg.Task.ID))
		return m, tea.Batch(cmds...)

	case components.CreateTaskDoneMsg:
		if !msg.Cancelled && msg.Title != "" {
			cmds = append(cmds, m.createTaskCmd(msg.Title, msg.Objective))
		}
		m.mode = ModeDashboard
		m.errorMsg = ""
		return m, tea.Batch(cmds...)

	case components.PlanModeMsg:
		m.mode = ModeDashboard
		if !msg.Cancelled && len(msg.Plans) > 0 {
			m.errorMsg = fmt.Sprintf("Creating %d sub-tasks...", len(msg.Plans))
			cmds = append(cmds, m.createSubTasksCmd(msg.TaskID, msg.Plans))
			cmds = append(cmds, m.loadTasks())
		}
		return m, tea.Batch(cmds...)

	case agentStartedMsg:
		m.activeTaskID = msg.taskID
		m.activeSubID = msg.subTaskID
		m.activePhase = msg.phase
		m.agentPanel.SetActive(msg.taskID, msg.subTaskID, msg.taskName, msg.phase)
		m.errorMsg = ""
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
		return m.handleAgentDone(msg, &cmds)

	case codingJudgeResultMsg:
		return m.handleCodingJudgeResult(msg, &cmds)

	case verifyJudgeResultMsg:
		return m.handleVerifyJudgeResult(msg, &cmds)

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

// handleAgentDone processes phase completion. For coding, it dispatches an OpenAI
// judge call to determine if the task was completed before auto-transitioning.
// For verification, it dispatches a judge call to check if verification passed.
func (m Model) handleAgentDone(msg components.AgentDoneMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	if msg.SubTaskID != m.activeSubID {
		var cmd tea.Cmd
		m.agentPanel, cmd = m.agentPanel.Update(msg)
		*cmds = append(*cmds, cmd)
		return m, tea.Batch(*cmds...)
	}

	taskObj := m.taskList.SelectedTask()
	var st *task.SubTask
	for _, s := range m.subTasks {
		if s.ID == msg.SubTaskID {
			st = s
			break
		}
	}

	if msg.Phase == task.PhaseCoding {
		// Coding done → ask OpenAI judge to determine if task is complete
		if st != nil && taskObj != nil {
			agentOutput := m.agentPanel.GetFullOutput()
			*cmds = append(*cmds, m.judgeCodingCmd(msg.SubTaskID, taskObj, st, agentOutput))
			m.errorMsg = "Coding done. Judging completion..."
		}
	} else if msg.Phase == task.PhaseVerifying {
		// Verification done → ask OpenAI judge to determine if it passed
		if st != nil && taskObj != nil {
			agentOutput := m.agentPanel.GetFullOutput()
			*cmds = append(*cmds, m.judgeVerificationCmd(msg.SubTaskID, taskObj, st, agentOutput))
			m.errorMsg = "Verification done. Judging results..."
		}
	}

	var cmd tea.Cmd
	m.agentPanel, cmd = m.agentPanel.Update(msg)
	*cmds = append(*cmds, cmd)
	return m, tea.Batch(*cmds...)
}

// judgeCodingCmd calls OpenAI to determine if the coding session completed its task.
func (m *Model) judgeCodingCmd(subTaskID string, taskObj *task.Task, st *task.SubTask, agentOutput string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		verdict, reason := task.JudgeCodingComplete(ctx, m.taskManager.GetLLMClient(), st.Title, st.Description, agentOutput)
		return codingJudgeResultMsg{
			taskID: taskObj.ID, subTaskID: subTaskID,
			verdict: verdict, reason: reason,
			taskObj: taskObj, st: st,
		}
	}
}

// judgeVerificationCmd calls OpenAI to determine if the verification passed.
func (m *Model) judgeVerificationCmd(subTaskID string, taskObj *task.Task, st *task.SubTask, agentOutput string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		verdict, reason := task.JudgeVerification(ctx, m.taskManager.GetLLMClient(), st.Title, agentOutput)
		return verifyJudgeResultMsg{
			taskID: taskObj.ID, subTaskID: subTaskID,
			verdict: verdict, reason: reason,
			taskObj: taskObj,
		}
	}
}

func (m Model) handleCodingJudgeResult(msg codingJudgeResultMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	m.errorMsg = fmt.Sprintf("Coding judge: %s — %s", msg.verdict, msg.reason)

	switch msg.verdict {
	case task.VerdictCompleted:
		// Coding completed successfully → auto-start verification
		if msg.st != nil && msg.taskObj != nil {
			*cmds = append(*cmds, m.runSubTaskVerifyCmd(msg.st, msg.taskObj))
			m.errorMsg = "Coding complete. Starting verification..."
		}

	case task.VerdictStuck:
		// Agent got stuck → mark as stuck, warn user
		m.activeSubID = ""
		m.activePhase = ""
		m.agentController.RemoveHandle(msg.subTaskID)
		m.agentPanel.SetInactive()
		for i, st := range m.subTasks {
			if st.ID == msg.subTaskID {
				m.subTasks[i].Status = task.SubTaskStuck
				break
			}
		}
		*cmds = append(*cmds, m.markSubTaskStuckCmd(msg.subTaskID))
		m.errorMsg = fmt.Sprintf("STUCK: %s — press 'r' to retry", msg.reason)

	case task.VerdictFailed:
		// Coding failed
		m.activeSubID = ""
		m.activePhase = ""
		m.agentController.RemoveHandle(msg.subTaskID)
		m.agentPanel.SetInactive()
		for i, st := range m.subTasks {
			if st.ID == msg.subTaskID {
				m.subTasks[i].Status = task.SubTaskFailed
				break
			}
		}
		*cmds = append(*cmds, m.markSubTaskFailedCmd(msg.subTaskID))
	}

	*cmds = append(*cmds, m.loadSubTasks(m.activeTaskID))
	return m, tea.Batch(*cmds...)
}

func (m Model) handleVerifyJudgeResult(msg verifyJudgeResultMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	m.activeSubID = ""
	m.activePhase = ""
	m.agentController.RemoveHandle(msg.subTaskID)
	m.agentPanel.SetInactive()

	switch msg.verdict {
	case task.VerifyPassed:
		// Verification passed → mark done and auto-start next
		for i, st := range m.subTasks {
			if st.ID == msg.subTaskID {
				m.subTasks[i].Status = task.SubTaskDone
				break
			}
		}
		*cmds = append(*cmds, m.markSubTaskDoneCmd(msg.subTaskID))
		m.errorMsg = fmt.Sprintf("Verification PASSED: %s", msg.reason)

		// Auto-start next planned or complete task
		if m.taskManager.SubTasks() != nil && msg.taskObj != nil {
			allDone := true
			for _, st := range m.subTasks {
				if !st.Status.IsTerminal() {
					allDone = false
					break
				}
			}
			if allDone {
				*cmds = append(*cmds, m.completeTaskCmd(msg.taskObj.ID))
				m.errorMsg = "All sub-tasks done. Task completed."
			} else {
				var next *task.SubTask
				for _, st := range m.subTasks {
					if st.Status == task.SubTaskPlanned {
						next = st
						break
					}
				}
				if next != nil {
					*cmds = append(*cmds, m.runSubTaskCodingCmd(next, msg.taskObj))
				}
			}
		}

	case task.VerifyFailed:
		// Verification found issues → mark failed
		for i, st := range m.subTasks {
			if st.ID == msg.subTaskID {
				m.subTasks[i].Status = task.SubTaskFailed
				break
			}
		}
		*cmds = append(*cmds, m.markSubTaskFailedCmd(msg.subTaskID))
		m.errorMsg = fmt.Sprintf("Verification FAILED: %s", msg.reason)
	}

	*cmds = append(*cmds, m.loadSubTasks(m.activeTaskID))
	return m, tea.Batch(*cmds...)
}

func (m *Model) markSubTaskStuckCmd(subTaskID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() != nil {
			_, _ = m.taskManager.SubTasks().TransitionStatus(ctx, subTaskID, task.SubTaskStuck)
		}
		return nil
	}
}

func (m *Model) markSubTaskFailedCmd(subTaskID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if m.taskManager.SubTasks() != nil {
			_, _ = m.taskManager.SubTasks().MarkFailed(ctx, subTaskID)
		}
		return nil
	}
}

// --- Dashboard key handler ---

func (m *Model) handleDashboardKey(msg tea.KeyMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.focusedPanel == 1 {
			m.taskList, _ = m.taskList.Update(msg)
			if t := m.taskList.SelectedTask(); t != nil {
				m.taskDetail.SetTask(t)
				m.subSelectedIdx = 0
				*cmds = append(*cmds, m.loadSubTasks(t.ID))
			}
		} else if m.focusedPanel == 2 {
			m.subTaskList.MoveUp()
			if st := m.subTaskList.SelectedSubTask(); st != nil {
				m.subDetail.SetSubTask(st)
			}
		}

	case "down", "j":
		if m.focusedPanel == 1 {
			m.taskList, _ = m.taskList.Update(msg)
			if t := m.taskList.SelectedTask(); t != nil {
				m.taskDetail.SetTask(t)
				m.subSelectedIdx = 0
				*cmds = append(*cmds, m.loadSubTasks(t.ID))
			}
		} else if m.focusedPanel == 2 {
			m.subTaskList.MoveDown()
			if st := m.subTaskList.SelectedSubTask(); st != nil {
				m.subDetail.SetSubTask(st)
			}
		}

	case "1":
		m.focusedPanel = 1
	case "2":
		m.focusedPanel = 2
	case "3":
		m.focusedPanel = 3
	case "tab":
		m.focusedPanel = (m.focusedPanel % 3) + 1

	case "n":
		m.mode = ModeCreateTask
		m.createForm = components.NewCreateTaskForm()
		*cmds = append(*cmds, m.createForm.Init())

	case "p":
		if t := m.taskList.SelectedTask(); t == nil {
			m.errorMsg = "No task selected \u2014 use arrow keys to select a task first"
		} else if t := m.taskList.SelectedTask(); t != nil {
			if !t.Status.IsActive() {
				m.errorMsg = "Task is not active (status: " + string(t.Status) + ")"
			} else {
				m.mode = ModePlan
				m.planMode = components.NewPlanMode(t.ID, t.Title)
				m.errorMsg = "Decomposing \"" + t.Title + "\" via ACP..."
				*cmds = append(*cmds, m.planTaskCmd(t))
			}
		}

	case "r":
		if t := m.taskList.SelectedTask(); t == nil {
			m.errorMsg = "No task selected \u2014 use arrow keys to select a task first"
		} else if t := m.taskList.SelectedTask(); t != nil {
			if m.activeSubID != "" {
				m.errorMsg = fmt.Sprintf("Agent already running for sub-task: %s", m.activeSubID)
			} else if m.taskManager.SubTasks() != nil {
				st := m.subTaskList.SelectedSubTask()
				if st == nil {
					next, _ := m.taskManager.SubTasks().NextPlanned(t.ID)
					st = next
				}
				if st != nil {
					if st.Status == task.SubTaskPlanned {
						m.errorMsg = fmt.Sprintf("Starting coding for \"%s\"...", st.Title)
						*cmds = append(*cmds, m.runSubTaskCodingCmd(st, t))
					} else {
						m.errorMsg = fmt.Sprintf("Sub-task status is %s, not planned", st.Status)
					}
				} else {
					m.errorMsg = "No sub-tasks. Press 'p' to plan first."
				}
			} else {
				m.errorMsg = "Sub-task manager not available"
			}
		}

	case "f":
		if t := m.taskList.SelectedTask(); t != nil {
			st := m.subTaskList.SelectedSubTask()
			if st != nil && m.activeSubID == "" {
				m.errorMsg = fmt.Sprintf("Starting verification for \"%s\"...", st.Title)
				*cmds = append(*cmds, m.runSubTaskVerifyCmd(st, t))
			} else {
				m.errorMsg = "No sub-task selected or agent already running"
			}
		}

	case "c":
		if m.activeSubID == "" {
			m.errorMsg = "No agent running."
		} else {
			if err := m.agentController.CancelAgent(m.activeSubID); err != nil {
				m.errorMsg = err.Error()
			} else {
				m.activeSubID = ""
				m.activePhase = ""
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

	case "enter":
		if m.focusedPanel == 1 {
			m.focusedPanel = 2
		}
	}
	return m, tea.Batch(*cmds...)
}

// --- Plan mode handler ---

func (m *Model) handlePlanModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeDashboard
		m.errorMsg = ""
		return m, nil
	}
	var cmd tea.Cmd
	m.planMode, cmd = m.planMode.Update(msg)
	return m, cmd
}

// --- Other mode handlers ---

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
	if m.mode == ModePlan {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.planMode.View())
	}

	runningCount := len(m.agentController.RunningSubTasks())
	statusBar := components.StatusBar(m.width, len(m.tasks), len(m.subTasks), runningCount, string(m.mode), "")
	statusLine := m.renderStatusLine()
	helpBar := m.renderHelpBar()

	taskListWidth := m.width / 4
	subDetailWidth := m.width - taskListWidth - 4
	topHeight := (m.height - 2) * 3 / 5
	bottomHeight := (m.height - 2) - topHeight

	// Task list
	taskBorder := "#45475A"
	if m.focusedPanel == 1 {
		taskBorder = "#89B4FA"
	}
	taskListStyled := lipgloss.NewStyle().Width(taskListWidth).Height(topHeight).
		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(taskBorder)).
		Render(m.taskList.View())

	// Sub-task list
	subListView := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CBA6F7")).Render("SubTasks") + "\n" + m.subTaskList.View()
	subBorder := "#45475A"
	if m.focusedPanel == 2 {
		subBorder = "#89B4FA"
	}
	subListStyled := lipgloss.NewStyle().Width(subDetailWidth/2).Height(topHeight).
		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(subBorder)).
		Render(subListView)

	// Sub-task detail
	subDetailStyled := lipgloss.NewStyle().Width(subDetailWidth-subDetailWidth/2-1).Height(topHeight).
		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475A")).
		Render(m.subDetail.View())

	rightPanel := lipgloss.JoinHorizontal(lipgloss.Top, subListStyled, subDetailStyled)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, taskListStyled, rightPanel)

	// Agent output
	agentBorder := "#45475A"
	if m.focusedPanel == 3 {
		agentBorder = "#89B4FA"
	}
	agentStyled := lipgloss.NewStyle().Width(m.width).Height(bottomHeight).
		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(agentBorder)).
		Render(m.agentPanel.View())

	mainArea := lipgloss.JoinVertical(lipgloss.Left, topRow, agentStyled)
	result := lipgloss.JoinVertical(lipgloss.Left, statusBar, mainArea, statusLine, helpBar)

	if m.mode == ModeConfirm {
		result = m.overlayConfirm()
	}
	if m.mode == ModeBindDir {
		result = m.overlayBindDir()
	}
	return result
}

func (m *Model) renderStatusLine() string {
	var msg string
	switch {
	case m.errorMsg != "":
		msg = m.errorMsg
	case m.activeSubID != "" && m.activePhase == task.PhaseCoding:
		msg = fmt.Sprintf("● Coding in progress... [c] cancel")
	case m.activeSubID != "" && m.activePhase == task.PhaseVerifying:
		msg = fmt.Sprintf("◷ Verifying in progress... [c] cancel")
	case m.activeSubID == "" && len(m.subTasks) > 0:
		sel := m.subTaskList.SelectedSubTask()
		if sel != nil && sel.Status == task.SubTaskStuck {
			msg = fmt.Sprintf("⚠ Stuck — press [r] to retry \"%s\"", sel.Title)
		} else if sel != nil && sel.Status == task.SubTaskPlanned {
			msg = fmt.Sprintf("▶ Ready — press [r] to run \"%s\"", sel.Title)
		} else {
			msg = "Press [p] to plan sub-tasks, [r] to run"
		}
	case len(m.subTasks) == 0 && len(m.tasks) > 0:
		sel := m.taskList.SelectedTask()
		if sel != nil && sel.Status.IsActive() {
			msg = fmt.Sprintf("Press [p] to plan — decompose \"%s\" into sub-tasks", sel.Title)
		} else {
			msg = "Press [n] to create a new task, [p] to plan"
		}
	default:
		msg = "Press [n] to create a new task"
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6ADC8")).
		Background(lipgloss.Color("#181825")).
		Padding(0, 1).
		Width(m.width)

	switch {
	case m.errorMsg != "":
		style = style.Foreground(lipgloss.Color("#F38BA8"))
	case m.activeSubID != "":
		style = style.Foreground(lipgloss.Color("#F9E2AF"))
	}

	return style.Render("  " + msg)
}


func (m *Model) renderHelpBar() string {
	return components.HelpBar(m.width, [][2]string{
		{"↑/↓", "navigate"}, {"tab", "panel"}, {"p", "plan"}, {"r", "run"},
		{"f", "verify"}, {"c", "cancel"}, {"n", "new"}, {"b", "bind"}, {"q", "quit"},
	})
}

func (m *Model) renderCreateForm() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.createForm.View())
}

func (m *Model) overlayConfirm() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#F9E2AF")).
			Background(lipgloss.Color("#1E1E2E")).Padding(1, 3).Render(m.errorMsg),
		lipgloss.WithWhitespaceChars(" "), lipgloss.WithWhitespaceForeground(lipgloss.Color("#11111B")))
}

func (m *Model) overlayBindDir() string {
	inputDisplay := m.bindInput
	if inputDisplay == "" {
		inputDisplay = "_"
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#89B4FA")).
			Background(lipgloss.Color("#1E1E2E")).Padding(1, 3).Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA")).Render("Bind Directory")+"\n\n"+
				m.errorMsg+"\n"+lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render("> "+inputDisplay)+"\n\n"+
				lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).Render("enter: confirm  esc: cancel")),
		lipgloss.WithWhitespaceChars(" "), lipgloss.WithWhitespaceForeground(lipgloss.Color("#11111B")))
}

// --- Layout ---

func (m *Model) initComponents() {
	taskListWidth := m.width / 4
	subRight := m.width - taskListWidth
	topH := (m.height - 2) * 3 / 5
	m.taskList = components.NewTaskList(taskListWidth, topH)
	m.taskDetail = components.NewTaskDetail(subRight, topH)
	m.subTaskList = components.NewSubTaskList(subRight/2, topH-3)
	m.subDetail = components.NewSubTaskDetail(subRight-subRight/2, topH-3)
	m.agentPanel = components.NewAgentPanel(m.width, (m.height-2)-topH)
	m.createForm = components.NewCreateTaskForm()
}

func (m *Model) resizeComponents() {
	taskListWidth := m.width / 4
	subRight := m.width - taskListWidth
	topH := (m.height - 2) * 3 / 5
	botH := (m.height - 2) - topH
	m.taskList.Resize(taskListWidth-4, topH-2)
	m.taskDetail.Resize(subRight-4, topH-2)
	m.subTaskList.Resize(subRight/2-3, topH-5)
	m.subDetail.Resize(subRight-subRight/2-3, topH-5)
	m.agentPanel.Resize(m.width-4, botH-2)
}

func (m *Model) selectedTaskID() string {
	if t := m.taskList.SelectedTask(); t != nil {
		return t.ID
	}
	return ""
}

func (m *Model) SetAcpManager(mgr *agent.AcpManager) { m.acpManager = mgr }
func (m *Model) GetAcpManager() *agent.AcpManager     { return m.acpManager }
func (m *Model) GetTaskManager() *task.TaskManager     { return m.taskManager }
func (m *Model) GetAgentController() *core.AgentController { return m.agentController }
