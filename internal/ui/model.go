package ui

import (
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/config"
	"cmdban/internal/markdown"
	"cmdban/internal/task"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeInput
	ModeCommand
	ModeConfirm
	ModeHelp
	ModeSettings
	ModeView
	ModeTag
	ModeSearch
)

type Model struct {
	board          *task.Board
	config         *config.Config
	keys           KeyMap
	help           help.Model
	textInput      textinput.Model
	mdRenderer     *markdown.Renderer
	width          int
	height         int
	activeLane     int
	activeTask     int
	viewScroll     int
	mode           Mode
	inputAction    string
	message        string
	gPressed       bool
	err            error
	tagSuggestions []string
	tagSuggestion  string
	searchFilter   string
}

type tasksLoadedMsg struct {
	tasks []*task.Task
}

type taskSavedMsg struct {
	task *task.Task
}

type errMsg struct {
	err error
}

type editorFinishedMsg struct {
	filePath string
}

type configEditorFinishedMsg struct{}

func NewModel(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter text..."
	ti.CharLimit = 256
	ti.Width = 50

	h := help.New()
	h.ShowAll = false

	mdStyles := markdown.DefaultStyles()
	mdRenderer := markdown.NewRenderer(mdStyles)

	return Model{
		board:      task.NewBoardWithHiddenColumns(cfg.HiddenColumns),
		config:     cfg,
		keys:       DefaultKeyMap,
		help:       h,
		textInput:  ti,
		mdRenderer: mdRenderer,
		activeLane: 0,
		activeTask: 0,
		mode:       ModeNormal,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadTasks
}

func (m Model) loadTasks() tea.Msg {
	tasks, err := task.LoadTasksFromDirectory(m.config.TaskDirectory)
	if err != nil {
		return errMsg{err}
	}
	return tasksLoadedMsg{tasks}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case tasksLoadedMsg:
		m.board = task.NewBoardWithHiddenColumns(m.config.HiddenColumns)
		for _, t := range msg.tasks {
			m.board.AddTask(t)
		}
		m.clampSelection()
		return m, nil

	case taskSavedMsg:
		return m, m.loadTasks

	case errMsg:
		m.err = msg.err
		m.message = "Error: " + msg.err.Error()
		return m, nil

	case editorFinishedMsg:
		return m, m.loadTasks

	case configEditorFinishedMsg:
		return m.reloadConfig()
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == ModeInput || m.mode == ModeCommand {
		return m.handleInputMode(msg)
	}

	if m.mode == ModeConfirm {
		return m.handleConfirmMode(msg)
	}

	if m.mode == ModeHelp {
		if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Escape) || msg.String() == "q" {
			m.mode = ModeNormal
		}
		return m, nil
	}

	if m.mode == ModeView {
		return m.handleViewMode(msg)
	}

	if m.mode == ModeTag {
		return m.handleTagMode(msg)
	}

	if m.mode == ModeSearch {
		return m.handleSearchMode(msg)
	}

	return m.handleNormalMode(msg)
}

func (m Model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		if m.searchFilter != "" {
			m.searchFilter = ""
			m.clampSelection()
			return m, nil
		}

	case key.Matches(msg, m.keys.Help):
		m.mode = ModeHelp
		m.help.ShowAll = true
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.activeTask--
		m.clampSelection()
		m.gPressed = false

	case key.Matches(msg, m.keys.Down):
		m.activeTask++
		m.clampSelection()
		m.gPressed = false

	case key.Matches(msg, m.keys.Left):
		m.activeLane--
		if m.activeLane < 0 {
			m.activeLane = len(m.board.Lanes) - 1
		}
		m.activeTask = 0
		m.clampSelection()
		m.gPressed = false

	case key.Matches(msg, m.keys.Right):
		m.activeLane++
		if m.activeLane >= len(m.board.Lanes) {
			m.activeLane = 0
		}
		m.activeTask = 0
		m.clampSelection()
		m.gPressed = false

	case key.Matches(msg, m.keys.Top):
		if m.gPressed {
			m.activeTask = 0
			m.gPressed = false
		} else {
			m.gPressed = true
		}

	case key.Matches(msg, m.keys.Bottom):
		lane := m.board.Lanes[m.activeLane]
		m.activeTask = len(m.board.Tasks[lane]) - 1
		if m.activeTask < 0 {
			m.activeTask = 0
		}
		m.gPressed = false

	case key.Matches(msg, m.keys.HalfUp):
		m.activeTask -= 5
		m.clampSelection()
		m.gPressed = false

	case key.Matches(msg, m.keys.HalfDown):
		m.activeTask += 5
		m.clampSelection()
		m.gPressed = false

	case key.Matches(msg, m.keys.MoveLeft):
		return m.moveTaskLeft()

	case key.Matches(msg, m.keys.MoveRight):
		return m.moveTaskRight()

	case key.Matches(msg, m.keys.New):
		m.mode = ModeInput
		m.inputAction = "new"
		m.textInput.Reset()
		m.textInput.Placeholder = "Task title..."
		m.textInput.Focus()
		m.gPressed = false
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Edit):
		t := m.selectedTask()
		if t != nil {
			m.mode = ModeInput
			m.inputAction = "edit"
			m.textInput.SetValue(t.Title)
			m.textInput.Focus()
			m.gPressed = false
			return m, textinput.Blink
		}

	case key.Matches(msg, m.keys.Delete):
		if m.selectedTask() != nil {
			m.mode = ModeConfirm
			m.inputAction = "delete"
			m.gPressed = false
		}

	case key.Matches(msg, m.keys.ToggleDone):
		return m.toggleTaskDone()

	case key.Matches(msg, m.keys.Tag):
		if m.selectedTask() != nil {
			m.mode = ModeTag
			m.textInput.Reset()
			m.textInput.Placeholder = "tag name..."
			m.textInput.Focus()
			m.tagSuggestions = m.collectAllTags()
			m.tagSuggestion = ""
			m.gPressed = false
			return m, textinput.Blink
		}

	case key.Matches(msg, m.keys.Mark):
		return m.toggleMark()

	case key.Matches(msg, m.keys.Settings):
		m.mode = ModeCommand
		m.inputAction = ""
		m.textInput.Reset()
		m.textInput.Placeholder = "command..."
		m.textInput.Focus()
		m.gPressed = false
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Search):
		m.mode = ModeSearch
		m.textInput.Reset()
		m.textInput.Placeholder = "search..."
		m.textInput.Focus()
		m.gPressed = false
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Refresh):
		m.gPressed = false
		return m, m.loadTasks

	case key.Matches(msg, m.keys.Enter):
		if m.selectedTask() != nil {
			m.mode = ModeView
			m.viewScroll = 0
			m.gPressed = false
		}
		return m, nil

	default:
		m.gPressed = false
	}

	return m, nil
}

func (m Model) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.mode = ModeNormal
		m.textInput.Reset()
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		value := strings.TrimSpace(m.textInput.Value())
		if value == "" {
			m.mode = ModeNormal
			return m, nil
		}

		switch m.inputAction {
		case "new":
			return m.createTask(value)
		case "edit":
			return m.editTask(value)
		case "settings":
			return m.handleSettingsCommand(value)
		}

		if m.mode == ModeCommand {
			return m.handleCommand(value)
		}

		m.mode = ModeNormal
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) handleConfirmMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.inputAction == "delete" {
			return m.deleteTask()
		}
		m.mode = ModeNormal
	case "n", "N", "esc":
		m.mode = ModeNormal
	}
	return m, nil
}

func (m Model) handleViewMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape), msg.String() == "q":
		m.mode = ModeNormal
		m.viewScroll = 0
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.viewScroll > 0 {
			m.viewScroll--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.viewScroll++
		return m, nil

	case key.Matches(msg, m.keys.Top):
		if m.gPressed {
			m.viewScroll = 0
			m.gPressed = false
		} else {
			m.gPressed = true
		}
		return m, nil

	case key.Matches(msg, m.keys.HalfUp):
		m.viewScroll -= 10
		if m.viewScroll < 0 {
			m.viewScroll = 0
		}
		return m, nil

	case key.Matches(msg, m.keys.HalfDown):
		m.viewScroll += 10
		return m, nil

	case key.Matches(msg, m.keys.MoveLeft):
		m.mode = ModeNormal
		return m.moveTaskLeft()

	case key.Matches(msg, m.keys.MoveRight):
		m.mode = ModeNormal
		return m.moveTaskRight()

	case key.Matches(msg, m.keys.Edit):
		t := m.selectedTask()
		if t != nil {
			m.mode = ModeInput
			m.inputAction = "edit"
			m.textInput.SetValue(t.Title)
			m.textInput.Focus()
			return m, textinput.Blink
		}

	case key.Matches(msg, m.keys.Delete):
		if m.selectedTask() != nil {
			m.mode = ModeConfirm
			m.inputAction = "delete"
		}
		return m, nil

	case key.Matches(msg, m.keys.ToggleDone):
		m.mode = ModeNormal
		return m.toggleTaskDone()

	case key.Matches(msg, m.keys.OpenEditor):
		t := m.selectedTask()
		if t != nil {
			return m, m.openInEditor(t.FilePath)
		}
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m, m.loadTasks

	default:
		m.gPressed = false
	}

	return m, nil
}

func (m Model) handleTagMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.mode = ModeNormal
		m.textInput.Reset()
		m.tagSuggestion = ""
		return m, nil

	case msg.String() == "tab":
		if m.tagSuggestion != "" {
			m.textInput.SetValue(m.tagSuggestion)
			m.textInput.SetCursor(len(m.tagSuggestion))
			m.tagSuggestion = ""
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		value := strings.TrimSpace(m.textInput.Value())
		if value == "" {
			m.mode = ModeNormal
			return m, nil
		}
		return m.addTagToTask(value)
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	m.tagSuggestion = m.findTagSuggestion(m.textInput.Value())
	return m, cmd
}

func (m Model) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.mode = ModeNormal
		m.searchFilter = ""
		m.textInput.Reset()
		m.clampSelection()
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		m.mode = ModeNormal
		m.searchFilter = strings.TrimSpace(m.textInput.Value())
		m.activeTask = 0
		m.clampSelection()
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	m.searchFilter = m.textInput.Value()
	m.activeTask = 0
	m.clampSelection()
	return m, cmd
}

func (m Model) filterTasks(tasks []*task.Task) []*task.Task {
	if m.searchFilter == "" {
		return tasks
	}
	filter := strings.ToLower(m.searchFilter)
	var filtered []*task.Task
	for _, t := range tasks {
		if strings.Contains(strings.ToLower(t.Title), filter) {
			filtered = append(filtered, t)
		}
		for _, tag := range t.Tags {
			if strings.Contains(strings.ToLower(tag), filter) {
				filtered = append(filtered, t)
				break
			}
		}
	}
	return filtered
}

func (m Model) collectAllTags() []string {
	tagSet := make(map[string]bool)
	for _, lane := range m.board.Lanes {
		for _, t := range m.board.Tasks[lane] {
			for _, tag := range t.Tags {
				tagSet[tag] = true
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

func (m Model) findTagSuggestion(input string) string {
	if input == "" {
		return ""
	}
	input = strings.ToLower(input)
	for _, tag := range m.tagSuggestions {
		if strings.HasPrefix(strings.ToLower(tag), input) && strings.ToLower(tag) != input {
			return tag
		}
	}
	return ""
}

func (m Model) addTagToTask(tag string) (tea.Model, tea.Cmd) {
	m.mode = ModeNormal
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	for _, existingTag := range t.Tags {
		if existingTag == tag {
			m.message = "Tag already exists"
			return m, nil
		}
	}

	t.Tags = append(t.Tags, tag)

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		m.mode = ModeNormal
		return m, nil
	}

	switch parts[0] {
	case "settings", "set", "s":
		if len(parts) > 1 && parts[1] == "dir" && len(parts) > 2 {
			m.config.TaskDirectory = parts[2]
			if err := m.config.Save(); err != nil {
				m.message = "Error saving config: " + err.Error()
			} else {
				m.message = "Task directory set to: " + parts[2]
				if err := m.config.EnsureTaskDirectory(); err != nil {
					m.message = "Error creating directory: " + err.Error()
				}
			}
			m.mode = ModeNormal
			return m, m.loadTasks
		}
		m.message = "Usage: settings dir <path>"

	case "q", "quit":
		return m, tea.Quit

	case "w", "write":
		m.message = "Tasks auto-saved to markdown files"

	case "help", "h":
		m.mode = ModeHelp
		m.help.ShowAll = true
		return m, nil

	case "config", "conf", "c":
		m.mode = ModeNormal
		return m, m.openConfigInEditor()

	default:
		m.message = "Unknown command: " + parts[0]
	}

	m.mode = ModeNormal
	return m, nil
}

func (m Model) handleSettingsCommand(value string) (tea.Model, tea.Cmd) {
	m.config.TaskDirectory = value
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
	} else {
		m.message = "Task directory set to: " + value
	}
	m.mode = ModeNormal
	return m, m.loadTasks
}

func (m Model) createTask(title string) (tea.Model, tea.Cmd) {
	m.mode = ModeNormal
	lane := m.board.Lanes[m.activeLane]

	return m, func() tea.Msg {
		t, err := task.CreateNewTask(m.config.TaskDirectory, title)
		if err != nil {
			return errMsg{err}
		}
		t.Status = lane
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) editTask(title string) (tea.Model, tea.Cmd) {
	m.mode = ModeNormal
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	return m, func() tea.Msg {
		t.Title = title
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) deleteTask() (tea.Model, tea.Cmd) {
	m.mode = ModeNormal
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	return m, func() tea.Msg {
		if err := task.DeleteTask(t); err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{}
	}
}

func (m Model) openInEditor(filePath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	c := exec.Command(editor, filePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{filePath: filePath}
	})
}

func (m Model) openConfigInEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	c := exec.Command(editor, m.config.ConfigPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return configEditorFinishedMsg{}
	})
}

func (m Model) reloadConfig() (tea.Model, tea.Cmd) {
	newConfig, err := config.Load()
	if err != nil {
		m.message = "Error reloading config: " + err.Error()
		return m, nil
	}
	m.config = newConfig
	m.message = "Config reloaded"

	m.board = task.NewBoardWithHiddenColumns(m.config.HiddenColumns)
	m.activeLane = 0
	m.activeTask = 0

	return m, m.loadTasks
}

func (m Model) moveTaskLeft() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || m.activeLane == 0 {
		return m, nil
	}

	newLane := m.board.Lanes[m.activeLane-1]
	m.board.MoveTask(t, newLane)
	m.activeLane--

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) moveTaskRight() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || m.activeLane >= len(m.board.Lanes)-1 {
		return m, nil
	}

	newLane := m.board.Lanes[m.activeLane+1]
	m.board.MoveTask(t, newLane)
	m.activeLane++

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) toggleTaskDone() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	var newStatus task.Status
	if t.Status == task.StatusDone {
		newStatus = task.StatusToday
	} else {
		newStatus = task.StatusDone
	}

	m.board.MoveTask(t, newStatus)

	for i, lane := range m.board.Lanes {
		if lane == newStatus {
			m.activeLane = i
			break
		}
	}
	m.clampSelection()

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) toggleMark() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	t.Marked = !t.Marked

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) selectedTask() *task.Task {
	if m.activeLane < 0 || m.activeLane >= len(m.board.Lanes) {
		return nil
	}
	lane := m.board.Lanes[m.activeLane]
	tasks := m.filterTasks(m.board.Tasks[lane])
	if m.activeTask < 0 || m.activeTask >= len(tasks) {
		return nil
	}
	return tasks[m.activeTask]
}

func (m *Model) clampSelection() {
	if m.activeLane < 0 {
		m.activeLane = 0
	}
	if m.activeLane >= len(m.board.Lanes) {
		m.activeLane = len(m.board.Lanes) - 1
	}

	lane := m.board.Lanes[m.activeLane]
	tasks := m.filterTasks(m.board.Tasks[lane])

	if m.activeTask < 0 {
		m.activeTask = 0
	}
	if len(tasks) > 0 && m.activeTask >= len(tasks) {
		m.activeTask = len(tasks) - 1
	}
	if len(tasks) == 0 {
		m.activeTask = 0
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var sections []string

	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderBoard())
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	title := TitleStyle.Render("╭─ KANBAN ─╮")

	var modeStr string
	switch m.mode {
	case ModeInput:
		modeStr = "[INSERT]"
	case ModeCommand:
		modeStr = "[COMMAND]"
	case ModeConfirm:
		modeStr = "[CONFIRM]"
	case ModeHelp:
		modeStr = "[HELP]"
	case ModeView:
		modeStr = "[VIEW]"
	case ModeTag:
		modeStr = "[TAG]"
	case ModeSearch:
		modeStr = "[SEARCH]"
	default:
		modeStr = "[NORMAL]"
	}

	mode := lipgloss.NewStyle().
		Foreground(HighlightColor).
		Bold(true).
		Render(modeStr)

	dir := StatusBarStyle.Render("📁 " + m.config.TaskDirectory)

	header := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", mode, "  ", dir)
	return header
}

func (m Model) renderBoard() string {
	if m.mode == ModeHelp {
		return m.renderHelp()
	}

	if m.mode == ModeView {
		return m.renderTaskDetail()
	}

	laneWidth := GetLaneWidth(m.width, len(m.board.Lanes))
	if laneWidth < 20 {
		laneWidth = 20
	}

	var lanes []string
	for i, status := range m.board.Lanes {
		lanes = append(lanes, m.renderLane(i, status, laneWidth))
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, lanes...)
	return board
}

func (m Model) renderLane(index int, status task.Status, width int) string {
	isActive := index == m.activeLane
	allTasks := m.board.Tasks[status]
	tasks := m.filterTasks(allTasks)

	headerStyle := InactiveLaneHeaderStyle
	laneStyle := LaneStyle
	if isActive {
		headerStyle = ActiveLaneHeaderStyle
		laneStyle = ActiveLaneStyle
	}

	statusName := strings.ToUpper(string(status))
	count := len(tasks)
	header := headerStyle.Width(width - 4).Render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			statusName,
			lipgloss.NewStyle().Foreground(SubtleColor).Render(
				" ("+strings.Repeat("●", min(count, 10))+")"),
		),
	)

	var taskViews []string
	taskViews = append(taskViews, header)

	availableHeight := m.height - 10
	if availableHeight < 5 {
		availableHeight = 5
	}

	for i, t := range tasks {
		isSelected := isActive && i == m.activeTask
		taskViews = append(taskViews, m.renderTask(t, isSelected, width-4))
	}

	if len(tasks) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(SubtleColor).
			Italic(true).
			Width(width - 4).
			Render("(empty)")
		taskViews = append(taskViews, empty)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, taskViews...)
	return laneStyle.Width(width).Height(availableHeight).Render(content)
}

func (m Model) renderTask(t *task.Task, selected bool, width int) string {
	isDone := t.Status == task.StatusDone

	var style lipgloss.Style
	switch {
	case selected && isDone:
		style = SelectedDoneTaskStyle
	case selected && t.Marked:
		style = SelectedTaskStyle.Foreground(lipgloss.Color("#AAAAAA"))
	case selected:
		style = SelectedTaskStyle
	case isDone:
		style = DoneTaskStyle
	case t.Marked:
		style = TaskStyle.Foreground(SubtleColor)
	default:
		style = TaskStyle
	}

	prefix := ""
	if t.Marked {
		prefix = "✓ "
	}

	title := prefix + t.Title
	if len(title) > width-2 {
		title = title[:width-5] + "..."
	}

	var tags string
	if len(t.Tags) > 0 {
		tagStrs := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tagStrs[i] = "@" + tag
		}
		tags = lipgloss.NewStyle().
			Foreground(SubtleColor).
			Render(" " + strings.Join(tagStrs, " "))
	}

	return style.Width(width).Render(title + tags)
}

func (m Model) renderFooter() string {
	if m.mode == ModeInput || m.mode == ModeCommand {
		prefix := "> "
		if m.mode == ModeCommand {
			prefix = ":"
		}
		input := InputStyle.Render(prefix + m.textInput.View())
		return input
	}

	if m.mode == ModeTag {
		inputValue := m.textInput.Value()
		suggestion := ""
		if m.tagSuggestion != "" && len(inputValue) > 0 {
			suggestion = lipgloss.NewStyle().
				Foreground(SubtleColor).
				Render(m.tagSuggestion[len(inputValue):] + " (tab)")
		}
		input := InputStyle.Render("@" + m.textInput.View() + suggestion)
		return input
	}

	if m.mode == ModeSearch {
		input := InputStyle.Render("/" + m.textInput.View())
		return input
	}

	if m.mode == ModeConfirm {
		return DialogStyle.Render("Delete task? (y/n)")
	}

	var parts []string

	if m.searchFilter != "" {
		filterMsg := lipgloss.NewStyle().
			Foreground(HighlightColor).
			Render("Filter: /" + m.searchFilter + "  (ESC to clear)")
		parts = append(parts, filterMsg)
	}

	if m.message != "" {
		parts = append(parts, StatusBarStyle.Foreground(WarningColor).Render(m.message))
	}

	helpView := m.help.View(m.keys)
	parts = append(parts, HelpStyle.Render(helpView))

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderTaskDetail() string {
	t := m.selectedTask()
	if t == nil {
		return "No task selected"
	}

	m.mdRenderer.SetWidth(m.width - 8)

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return DialogStyle.Render("Error reading task: " + err.Error())
	}

	rendered := m.mdRenderer.Render(string(content))

	lines := strings.Split(rendered, "\n")
	availableHeight := m.height - 8

	if m.viewScroll >= len(lines) {
		m.viewScroll = len(lines) - 1
	}
	if m.viewScroll < 0 {
		m.viewScroll = 0
	}

	endLine := m.viewScroll + availableHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	visibleLines := lines[m.viewScroll:endLine]
	visibleContent := strings.Join(visibleLines, "\n")

	statusLine := lipgloss.NewStyle().
		Foreground(SubtleColor).
		Render(strings.Repeat("─", m.width-8))

	tags := ""
	if len(t.Tags) > 0 {
		tagStrs := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tagStrs[i] = "@" + tag
		}
		tags = lipgloss.NewStyle().
			Foreground(HighlightColor).
			Render(strings.Join(tagStrs, " "))
	}

	footer := lipgloss.NewStyle().
		Foreground(SubtleColor).
		Render("Status: " + string(t.Status) + "  " + tags + "  [q: back, j/k: scroll, ctrl+g: edit in nvim]")

	detailStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveColor).
		Padding(1, 2).
		Width(m.width - 4).
		Height(availableHeight + 2)

	return detailStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			visibleContent,
			statusLine,
			footer,
		),
	)
}

func (m Model) renderHelp() string {
	help := `
  NAVIGATION                    ACTIONS
  ──────────                    ───────
  h/←      left lane            a       new task
  l/→      right lane           e/i     edit task
  j/↓      down                 d       toggle done
  k/↑      up                   ctrl+d  delete task
  gg       top of lane          t       add tag
  G        bottom of lane       m       mark task
  ctrl+u   half page up         H       move task left
  ctrl+f   half page down       L       move task right
                                enter   view task

  MISC                          COMMANDS
  ────                          ────────
  ?        toggle help          :       command mode
  r        refresh              :q      quit
  q        quit

  Press ? or ESC to close help
`
	return DialogStyle.Width(60).Render(help)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
