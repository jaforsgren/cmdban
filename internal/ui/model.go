package ui

import (
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/applog"
	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
	"cmdban/internal/markdown"
	"cmdban/internal/pat"
	"cmdban/internal/task"
)

var urlRegex = regexp.MustCompile(`https?://[^\s)\]]+`)

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
	ModeBoardSelector
	ModePATManager
	ModeADOSetup
	ModeLogs
)

type Model struct {
	board              *task.Board
	config             *config.Config
	patStore           *pat.Store
	keys               KeyMap
	help               help.Model
	textInput          textinput.Model
	mdRenderer         *markdown.Renderer
	width              int
	height             int
	activeLane         int
	activeTask         int
	viewScroll         int
	mode               Mode
	inputAction        string
	message            string
	gPressed           bool
	err                error
	tagSuggestions     []string
	tagSuggestion      string
	searchFilter       string
	boardSelectorIndex int
	pendingBoardName   string
	patSelectorIndex   int
	pendingPATName     string
	revealedPATIndex   int
	lastSyncTime       time.Time
	adoSetup           adoSetupState
	minimizedLanes     map[task.Status]bool
	logs               *applog.Buffer
	logScroll          int
}

type tasksLoadedMsg struct {
	tasks []*task.Task
}

type taskSavedMsg struct {
	task *task.Task
}

type adoStateUpdatedMsg struct{}

type errMsg struct {
	err error
}

type taskOrderSavedMsg struct{}

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

	patStore, err := pat.Load()
	if err != nil {
		patStore = &pat.Store{}
	}

	return Model{
		board:            task.NewBoardWithColumns(cfg.AllColumns(), cfg.HiddenColumns()),
		config:           cfg,
		patStore:         patStore,
		keys:             DefaultKeyMap,
		help:             h,
		textInput:        ti,
		mdRenderer:       mdRenderer,
		activeLane:       0,
		activeTask:       0,
		mode:             ModeNormal,
		revealedPATIndex: -1,
		minimizedLanes:   make(map[task.Status]bool),
		logs:             applog.NewBuffer(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadTasks
}

func (m Model) loadTasks() tea.Msg {
	if m.config.IsADOBoard() {
		return m.loadADOTasks()
	}
	tasks, err := task.LoadTasksFromDirectory(m.config.TaskDirectory())
	if err != nil {
		return errMsg{err}
	}
	return tasksLoadedMsg{tasks}
}

func (m Model) loadADOTasks() tea.Msg {
	adoCfg := m.config.ActiveBoard().AzureDevOps
	token, ok := m.patStore.Get(adoCfg.PAT)
	if !ok {
		return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
	}
	tasks, diag, err := azuredevops.FetchBoardTasks(adoCfg, token)
	if err != nil {
		return errMsg{err}
	}
	m.logs.Info(fmt.Sprintf(
		"ADO sync %s/%s/%s source=%s iteration=%q: fetched %d, showing %d",
		adoCfg.Org, adoCfg.Project, adoCfg.Team, diag.Source, diag.IterationPath, diag.FetchedCount, diag.ReturnedCount,
	))
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
		m.board = task.NewBoardWithColumns(m.config.AllColumns(), m.config.HiddenColumns())
		for _, t := range msg.tasks {
			m.board.AddTask(t)
		}
		m.clampSelection()
		if m.config.IsADOBoard() {
			m.lastSyncTime = time.Now()
			m.message = ""
		}
		return m, nil

	case taskSavedMsg:
		return m, m.loadTasks

	case adoStateUpdatedMsg:
		return m, nil

	case adoOrgsDiscoveredMsg:
		return m.handleADOOrgsDiscovered(msg.orgs)

	case adoOrgsDiscoveryFailedMsg:
		return m.handleADOOrgsDiscoveryFailed(msg.err)

	case adoProjectsDiscoveredMsg:
		m.adoSetup.loading = false
		m.adoSetup.projects = msg.projects
		m.adoSetup.selectorIndex = 0
		return m, nil

	case adoTeamsDiscoveredMsg:
		m.adoSetup.loading = false
		m.adoSetup.teams = msg.teams
		m.adoSetup.selectorIndex = bestMatchIndex(msg.teams, m.adoSetup.team)
		return m, nil

	case adoBoardColumnsDiscoveredMsg:
		return m.handleADOBoardColumnsDiscovered(msg.columns)

	case adoIterationsDiscoveredMsg:
		return m.handleADOIterationsDiscovered(msg.iterations)

	case errMsg:
		m.err = msg.err
		m.message = "Error: " + msg.err.Error()
		m.logs.Error(msg.err.Error())
		return m, nil

	case taskOrderSavedMsg:
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

	if m.mode == ModeBoardSelector {
		return m.handleBoardSelectorMode(msg)
	}

	if m.mode == ModePATManager {
		return m.handlePATManagerMode(msg)
	}

	if m.mode == ModeADOSetup {
		return m.handleADOSetupMode(msg)
	}

	if m.mode == ModeLogs {
		return m.handleLogsMode(msg)
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

	case key.Matches(msg, m.keys.OrderUp):
		return m.reorderTask(-1)

	case key.Matches(msg, m.keys.OrderDown):
		return m.reorderTask(1)

	case key.Matches(msg, m.keys.New):
		m.mode = ModeInput
		m.inputAction = "new"
		m.textInput.Reset()
		m.textInput.Placeholder = "Task title..."
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		m.gPressed = false
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Edit):
		t := m.selectedTask()
		if t != nil {
			m.mode = ModeInput
			m.inputAction = "edit"
			m.textInput.SetValue(t.Title)
			m.textInput.EchoMode = textinput.EchoNormal
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
			if m.config.IsADOBoard() {
				m.message = "Tags are managed in Azure DevOps"
				return m, nil
			}
			m.mode = ModeTag
			m.textInput.Reset()
			m.textInput.Placeholder = "tag name..."
			m.textInput.EchoMode = textinput.EchoNormal
			m.textInput.Focus()
			m.tagSuggestions = m.collectAllTags()
			m.tagSuggestion = ""
			m.gPressed = false
			return m, textinput.Blink
		}

	case key.Matches(msg, m.keys.Mark):
		if m.config.IsADOBoard() {
			m.message = "Marking is not supported for Azure DevOps boards"
			return m, nil
		}
		return m.toggleMark()

	case key.Matches(msg, m.keys.Settings):
		m.mode = ModeCommand
		m.inputAction = ""
		m.textInput.Reset()
		m.textInput.Placeholder = "command..."
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		m.gPressed = false
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Search):
		m.mode = ModeSearch
		m.textInput.Reset()
		m.textInput.Placeholder = "search..."
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		m.gPressed = false
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Refresh):
		m.gPressed = false
		if m.config.IsADOBoard() {
			m.message = "Syncing with Azure DevOps..."
		}
		return m, m.loadTasks

	case key.Matches(msg, m.keys.Boards):
		m.boardSelectorIndex = m.activeBoardIndex()
		m.mode = ModeBoardSelector
		m.gPressed = false
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.selectedTask() != nil {
			m.mode = ModeView
			m.viewScroll = 0
			m.gPressed = false
		}
		return m, nil

	case key.Matches(msg, m.keys.CollapseColumn):
		lane := m.board.Lanes[m.activeLane]
		m.minimizedLanes[lane] = !m.minimizedLanes[lane]
		m.gPressed = false
		return m, nil

	case key.Matches(msg, m.keys.OpenURL):
		m.gPressed = false
		return m.openTaskURL()

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
		m.textInput.EchoMode = textinput.EchoNormal
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		value := strings.TrimSpace(m.textInput.Value())
		if value == "" {
			m.mode = ModeNormal
			m.textInput.EchoMode = textinput.EchoNormal
			return m, nil
		}

		switch m.inputAction {
		case "new":
			return m.createTask(value)
		case "edit":
			return m.editTask(value)
		case "settings":
			return m.handleSettingsCommand(value)
		case "new-board-name":
			m.pendingBoardName = value
			m.inputAction = "new-board-dir"
			m.textInput.Reset()
			m.textInput.Placeholder = "Directory path..."
			return m, textinput.Blink
		case "new-board-dir":
			return m.createBoard(m.pendingBoardName, value)
		case "pat-name":
			m.pendingPATName = value
			m.inputAction = "pat-token"
			m.textInput.Reset()
			m.textInput.Placeholder = "PAT token..."
			m.textInput.EchoMode = textinput.EchoPassword
			return m, textinput.Blink
		case "pat-token":
			m.patStore.Set(m.pendingPATName, value)
			if err := m.patStore.Save(); err != nil {
				m.message = "Error saving PAT: " + err.Error()
				m.logs.Error(m.message)
			} else {
				m.message = fmt.Sprintf("PAT %q saved", m.pendingPATName)
			}
			m.mode = ModePATManager
			m.textInput.Reset()
			m.textInput.EchoMode = textinput.EchoNormal
			return m, nil
		case "ado-url":
			if value == "" {
				m.mode = ModeADOSetup
				return m, nil
			}
			org, project, team, boardLevel, isBacklog, err := parseADOBoardURL(value)
			if err != nil {
				m.message = "Invalid ADO URL: " + err.Error()
				m.mode = ModeNormal
				return m, nil
			}
			m.adoSetup.org = org
			m.adoSetup.project = project
			m.adoSetup.team = team
			m.adoSetup.boardLevel = boardLevel
			m.adoSetup.isBacklog = isBacklog
			m.adoSetup.boardName = strings.ToLower(strings.ReplaceAll(project, " ", "-")) + "-sprint"
			m.adoSetup.fromURL = true
			m.mode = ModeADOSetup
			return m, nil
		case "ado-org-name":
			m.adoSetup.org = value
			m.adoSetup.loadError = ""
			m.adoSetup.loading = true
			m.adoSetup.step = adoSetupStepSelectProject
			m.adoSetup.selectorIndex = 0
			m.mode = ModeADOSetup
			m.textInput.Reset()
			m.textInput.EchoMode = textinput.EchoNormal
			return m, m.discoverProjects
		case "ado-board-name":
			return m.saveADOBoard(value)
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

	case key.Matches(msg, m.keys.OrderUp):
		return m.reorderTask(-1)

	case key.Matches(msg, m.keys.OrderDown):
		return m.reorderTask(1)

	case key.Matches(msg, m.keys.Edit):
		t := m.selectedTask()
		if t != nil {
			m.mode = ModeInput
			m.inputAction = "edit"
			m.textInput.SetValue(t.Title)
			m.textInput.EchoMode = textinput.EchoNormal
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
			if m.config.IsADOBoard() {
				m.message = "No local file for Azure DevOps work items"
				return m, nil
			}
			return m, m.openInEditor(t.FilePath)
		}
		return m, nil

	case key.Matches(msg, m.keys.OpenURL):
		return m.openTaskURL()

	case key.Matches(msg, m.keys.Refresh):
		if m.config.IsADOBoard() {
			m.message = "Syncing with Azure DevOps..."
		}
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

func (m Model) handlePATManagerMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pats := m.patStore.PATs
	switch msg.String() {
	case "j", "down":
		m.patSelectorIndex++
		if m.patSelectorIndex >= len(pats) {
			m.patSelectorIndex = 0
		}
		m.revealedPATIndex = -1

	case "k", "up":
		m.patSelectorIndex--
		if m.patSelectorIndex < 0 {
			m.patSelectorIndex = max(0, len(pats)-1)
		}
		m.revealedPATIndex = -1

	case " ":
		if m.revealedPATIndex == m.patSelectorIndex {
			m.revealedPATIndex = -1
		} else {
			m.revealedPATIndex = m.patSelectorIndex
		}

	case "a":
		m.mode = ModeInput
		m.inputAction = "pat-name"
		m.textInput.Reset()
		m.textInput.Placeholder = "PAT name (e.g. myorg)..."
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		return m, textinput.Blink

	case "x", "ctrl+d":
		if m.patSelectorIndex < len(pats) {
			name := pats[m.patSelectorIndex].Name
			m.patStore.Delete(name)
			if err := m.patStore.Save(); err != nil {
				m.message = "Error saving: " + err.Error()
				m.logs.Error(m.message)
			} else {
				m.message = fmt.Sprintf("PAT %q deleted", name)
			}
			if m.patSelectorIndex >= len(m.patStore.PATs) {
				m.patSelectorIndex = max(0, len(m.patStore.PATs)-1)
			}
			m.revealedPATIndex = -1
		}

	case "esc", "q":
		m.mode = ModeNormal
		m.revealedPATIndex = -1
	}
	return m, nil
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
			continue
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
			if board := m.config.ActiveBoard(); board != nil {
				board.Directory = parts[2]
			}
			if err := m.config.Save(); err != nil {
				m.message = "Error saving config: " + err.Error()
				m.logs.Error(m.message)
			} else {
				m.message = "Task directory set to: " + parts[2]
				if err := m.config.EnsureTaskDirectory(); err != nil {
					m.message = "Error creating directory: " + err.Error()
					m.logs.Error(m.message)
				}
			}
			m.mode = ModeNormal
			return m, m.loadTasks
		}
		m.message = "Usage: settings dir <path>"

	case "pats", "p":
		m.patSelectorIndex = 0
		m.revealedPATIndex = -1
		m.mode = ModePATManager
		m.message = ""
		m.mode = ModePATManager
		return m, nil

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

	case "boards", "board", "b":
		m.boardSelectorIndex = m.activeBoardIndex()
		m.mode = ModeBoardSelector
		return m, nil

	case "logs", "log", "l":
		m.logScroll = 0
		m.mode = ModeLogs
		return m, nil

	default:
		m.message = "Unknown command: " + parts[0]
	}

	m.mode = ModeNormal
	return m, nil
}

func (m Model) handleSettingsCommand(value string) (tea.Model, tea.Cmd) {
	if board := m.config.ActiveBoard(); board != nil {
		board.Directory = value
	}
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
	} else {
		m.message = "Task directory set to: " + value
	}
	m.mode = ModeNormal
	return m, m.loadTasks
}

func (m Model) createTask(title string) (tea.Model, tea.Cmd) {
	m.mode = ModeNormal
	lane := m.board.Lanes[m.activeLane]

	if m.config.IsADOBoard() {
		return m, m.createADOTask(title, lane)
	}

	return m, func() tea.Msg {
		t, err := task.CreateNewTask(m.config.TaskDirectory(), title)
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

func (m Model) createADOTask(title string, lane task.Status) tea.Cmd {
	adoCfg := m.config.ActiveBoard().AzureDevOps
	return func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		t, err := azuredevops.CreateRemoteTask(adoCfg, token, title, lane)
		if err != nil {
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

	if m.config.IsADOBoard() {
		adoCfg := m.config.ActiveBoard().AzureDevOps
		snapshot := t
		return m, func() tea.Msg {
			token, ok := m.patStore.Get(adoCfg.PAT)
			if !ok {
				return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
			}
			if err := azuredevops.UpdateRemoteTitle(adoCfg, token, snapshot, title); err != nil {
				return errMsg{err}
			}
			snapshot.Title = title
			return taskSavedMsg{snapshot}
		}
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
	if m.config.IsADOBoard() {
		m.message = "Use Azure DevOps to delete work items"
		return m, nil
	}
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	return m, func() tea.Msg {
		if err := task.ArchiveTask(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{}
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

func (m Model) openTaskURL() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	url, ok := m.taskURL(t)
	if !ok {
		m.message = "No URL found for this task"
		return m, nil
	}

	if err := openURLInBrowser(url); err != nil {
		m.message = "Failed to open URL: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}

	m.message = "Opened " + url
	return m, nil
}

// taskURL resolves the URL to open for a task: for Azure DevOps boards it is
// the constructed work item link, otherwise it is the first http(s) URL
// found in the task description.
func (m Model) taskURL(t *task.Task) (string, bool) {
	if m.config.IsADOBoard() {
		board := m.config.ActiveBoard()
		if board == nil || board.AzureDevOps == nil || t.ADOItemID == 0 {
			return "", false
		}
		return fmt.Sprintf("https://dev.azure.com/%s/%s/_workitems/edit/%d",
			board.AzureDevOps.Org, board.AzureDevOps.Project, t.ADOItemID), true
	}

	match := urlRegex.FindString(t.Description)
	if match == "" {
		return "", false
	}
	return match, true
}

func openURLInBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}

func (m Model) openConfigInEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	c := exec.Command(editor, m.config.ConfigPath())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return configEditorFinishedMsg{}
	})
}

func (m Model) reloadConfig() (tea.Model, tea.Cmd) {
	newConfig, err := config.Load()
	if err != nil {
		m.message = "Error reloading config: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	m.config = newConfig
	m.message = "Config reloaded"

	m.board = task.NewBoardWithColumns(m.config.AllColumns(), m.config.HiddenColumns())
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
	m.clampSelection()

	if m.config.IsADOBoard() {
		return m, m.pushADOStateChange(t, newLane)
	}

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
	m.clampSelection()

	if m.config.IsADOBoard() {
		return m, m.pushADOStateChange(t, newLane)
	}

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) pushADOStateChange(t *task.Task, newLane task.Status) tea.Cmd {
	adoCfg := m.config.ActiveBoard().AzureDevOps
	snapshot := t
	return func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		if err := azuredevops.PushStateChange(adoCfg, token, snapshot, newLane); err != nil {
			return errMsg{err}
		}
		return adoStateUpdatedMsg{}
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

	if m.config.IsADOBoard() {
		return m, m.pushADOStateChange(t, newStatus)
	}

	return m, func() tea.Msg {
		if err := task.WriteMarkdownFile(t); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{t}
	}
}

func (m Model) reorderTask(delta int) (tea.Model, tea.Cmd) {
	if m.config.IsADOBoard() {
		m.message = "Reordering is not supported for Azure DevOps boards"
		return m, nil
	}

	t := m.selectedTask()
	if t == nil {
		return m, nil
	}

	lane := t.Status
	tasks := m.board.Tasks[lane]

	idx := -1
	for i, task := range tasks {
		if task.ID == t.ID {
			idx = i
			break
		}
	}

	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(tasks) {
		return m, nil
	}

	tasks[idx], tasks[newIdx] = tasks[newIdx], tasks[idx]
	m.board.Tasks[lane] = tasks

	if m.searchFilter == "" {
		m.activeTask = newIdx
	}

	return m, m.saveLaneOrder(lane)
}

func (m Model) saveLaneOrder(lane task.Status) tea.Cmd {
	snapshot := make([]*task.Task, len(m.board.Tasks[lane]))
	copy(snapshot, m.board.Tasks[lane])

	return func() tea.Msg {
		for i, t := range snapshot {
			t.Order = i + 1
			if err := task.WriteMarkdownFile(t); err != nil {
				return errMsg{err}
			}
		}
		return taskOrderSavedMsg{}
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
	case ModeBoardSelector:
		modeStr = "[BOARDS]"
	case ModePATManager:
		modeStr = "[PATS]"
	case ModeADOSetup:
		modeStr = "[ADO SETUP]"
	case ModeLogs:
		modeStr = "[LOGS]"
	default:
		modeStr = "[NORMAL]"
	}

	mode := lipgloss.NewStyle().
		Foreground(HighlightColor).
		Bold(true).
		Render(modeStr)

	boardName := ""
	if board := m.config.ActiveBoard(); board != nil {
		name := board.Name
		if board.Type == config.BoardTypeAzureDevOps {
			name += " [ADO]"
		}
		boardName = lipgloss.NewStyle().Foreground(HighlightColor).Render(" [" + name + "]")
	}

	var statusPart string
	if m.config.IsADOBoard() {
		if !m.lastSyncTime.IsZero() {
			elapsed := time.Since(m.lastSyncTime).Round(time.Second)
			statusPart = lipgloss.NewStyle().Foreground(SubtleColor).Render(fmt.Sprintf("  synced %s ago", elapsed))
		}
	} else {
		statusPart = StatusBarStyle.Render("📁 " + m.config.TaskDirectory())
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center, title, boardName, "  ", mode, statusPart)
	return header
}

func (m Model) renderBoard() string {
	if m.mode == ModeHelp {
		return m.renderHelp()
	}

	if m.mode == ModeView {
		return m.renderTaskDetail()
	}

	if m.mode == ModeBoardSelector {
		return m.renderBoardSelector()
	}

	if m.mode == ModePATManager {
		return m.renderPATManager()
	}

	if m.mode == ModeADOSetup {
		return m.renderADOSetup()
	}

	if m.mode == ModeLogs {
		return m.renderLogs()
	}

	laneWidths := m.computeLaneWidths()

	var lanes []string
	for i, status := range m.board.Lanes {
		minimized := m.minimizedLanes[status]
		lanes = append(lanes, m.renderLane(i, status, laneWidths[i], minimized))
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, lanes...)
	return board
}

func (m Model) computeLaneWidths() []int {
	const laneOverhead = 4 // border(2) + padding(2) per lane
	widths := make([]int, len(m.board.Lanes))

	minimizedOuterTotal := 0
	for i, status := range m.board.Lanes {
		if m.minimizedLanes[status] {
			statusName := strings.ToUpper(string(status))
			// Header text sits inside the lane's Width(width-4) box, which itself
			// has 2 cols of horizontal padding (LaneHeaderStyle), so the text area
			// is width-6; it must be >= len(statusName) or the header wraps.
			w := len(statusName) + 6
			widths[i] = w
			minimizedOuterTotal += w + laneOverhead
		}
	}

	nonMinimizedCount := 0
	for _, status := range m.board.Lanes {
		if !m.minimizedLanes[status] {
			nonMinimizedCount++
		}
	}

	if nonMinimizedCount == 0 {
		return widths
	}

	remainingWidth := m.width - minimizedOuterTotal
	normalWidth := GetLaneWidth(remainingWidth, nonMinimizedCount)
	if normalWidth < 20 {
		normalWidth = 20
	}

	for i, status := range m.board.Lanes {
		if !m.minimizedLanes[status] {
			widths[i] = normalWidth
		}
	}

	return widths
}

func (m Model) renderLane(index int, status task.Status, width int, minimized bool) string {
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

	// 2 for lane border (top+bottom), 2 for app header+footer (conservative)
	availableHeight := m.height - 4
	if availableHeight < 5 {
		availableHeight = 5
	}

	if minimized {
		header := headerStyle.Width(width - 4).Render(statusName)
		return laneStyle.Width(width).Height(availableHeight).Render(header)
	}

	header := headerStyle.Width(width - 4).Render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			statusName,
			lipgloss.NewStyle().Foreground(SubtleColor).Render(
				" ("+strings.Repeat("●", min(count, 10))+")"),
		),
	)

	headerHeight := lipgloss.Height(header)
	// Reserve 2 lines for the ▲/▼ scroll indicators so they never push content over budget.
	contentHeight := availableHeight - headerHeight - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	var taskViews []string
	taskViews = append(taskViews, header)

	if len(tasks) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(SubtleColor).
			Italic(true).
			Width(width - 4).
			Render("(empty)")
		taskViews = append(taskViews, empty)
	} else {
		groupColors := tagGroupColors(tasks)
		rendered := make([]string, len(tasks))
		for i, t := range tasks {
			isSelected := isActive && i == m.activeTask
			rendered[i] = m.renderTask(t, isSelected, width-4, groupColors[i])
		}

		activeIdx := 0
		if isActive {
			activeIdx = m.activeTask
		}
		visible, above, below := laneScrollWindow(rendered, activeIdx, contentHeight)
		if above > 0 {
			taskViews = append(taskViews, lipgloss.NewStyle().
				Foreground(SubtleColor).
				Width(width-4).
				Render(fmt.Sprintf("  ▲ %d more", above)))
		}
		taskViews = append(taskViews, visible...)
		if below > 0 {
			taskViews = append(taskViews, lipgloss.NewStyle().
				Foreground(SubtleColor).
				Width(width-4).
				Render(fmt.Sprintf("  ▼ %d more", below)))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, taskViews...)
	// Hard-clamp: lipgloss .Height() pads but never clips, so truncate here.
	if lines := strings.Split(content, "\n"); len(lines) > availableHeight {
		content = strings.Join(lines[:availableHeight], "\n")
	}
	return laneStyle.Width(width).Height(availableHeight).Render(content)
}

// laneScrollWindow returns the slice of rendered task strings that fit within
// availableHeight lines, centred around activeIdx. It also returns the number
// of items hidden above and below the window.
func laneScrollWindow(rendered []string, activeIdx, availableHeight int) (visible []string, above, below int) {
	if availableHeight <= 0 {
		return rendered, 0, 0
	}

	heights := make([]int, len(rendered))
	for i, s := range rendered {
		h := lipgloss.Height(s)
		if h < 1 {
			h = 1
		}
		heights[i] = h
	}

	// Seed window with the active item.
	start := activeIdx
	end := activeIdx + 1
	used := heights[activeIdx]

	// Expand backward then forward greedily.
	for start > 0 && used+heights[start-1] <= availableHeight {
		start--
		used += heights[start]
	}
	for end < len(rendered) && used+heights[end] <= availableHeight {
		used += heights[end]
		end++
	}

	return rendered[start:end], start, len(rendered) - end
}

// tagGroupColors returns, for each task in order, the color of the vertical
// group bar to render beside it. Adjacent tasks sharing an ADO parent (or,
// absent that, a tag) are considered one group and get a matching bar color;
// a task with no adjacent match gets an empty color (no bar). Grouping is
// purely visual and does not affect ordering or selection.
func tagGroupColors(tasks []*task.Task) []lipgloss.Color {
	colors := make([]lipgloss.Color, len(tasks))
	for i := range tasks {
		var key string
		if i > 0 {
			key = sharedGroupKey(tasks[i-1], tasks[i])
		}
		if key == "" && i < len(tasks)-1 {
			key = sharedGroupKey(tasks[i], tasks[i+1])
		}
		if key != "" {
			colors[i] = tagBarColor(key)
		}
	}
	return colors
}

// ungroupableTags are work-item-type tags that are too common to signal a
// meaningful visual grouping (most tasks in a lane may share one).
var ungroupableTags = map[string]bool{
	"user-story": true,
	"feature":    true,
	"epic":       true,
}

// sharedGroupKey returns a key identifying the group both tasks belong to,
// or "" if none. ADO work items sharing a parent take priority over tasks
// merely sharing a tag, since the parent link is an explicit relationship
// rather than an incidental match.
func sharedGroupKey(a, b *task.Task) string {
	if a.ADOParentID != 0 && a.ADOParentID == b.ADOParentID {
		return fmt.Sprintf("parent-%d", a.ADOParentID)
	}
	return sharedTag(a, b)
}

// sharedTag returns a tag common to both tasks, or "" if none.
func sharedTag(a, b *task.Task) string {
	for _, ta := range a.Tags {
		if ungroupableTags[ta] {
			continue
		}
		for _, tb := range b.Tags {
			if ta == tb {
				return ta
			}
		}
	}
	return ""
}

var tagBarPalette = []lipgloss.Color{
	lipgloss.Color("#db6a39"),
	lipgloss.Color("#38b555"),
	lipgloss.Color("#4a9eda"),
	lipgloss.Color("#c39bd3"),
	lipgloss.Color("#e6c229"),
}

// tagBarColor deterministically maps a tag name to a palette color so the
// same tag always renders with the same group bar color.
func tagBarColor(tag string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(tag))
	return tagBarPalette[h.Sum32()%uint32(len(tagBarPalette))]
}

func (m Model) renderTask(t *task.Task, selected bool, width int, groupColor lipgloss.Color) string {
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

	// Reserve the last column for the tag-group bar so all rows in a lane
	// stay aligned whether or not they carry a bar.
	contentWidth := width - 1

	prefix := ""
	if t.Marked {
		prefix = "✓ "
	}

	adoBadge := ""
	if t.ADOItemID > 0 {
		adoBadge = lipgloss.NewStyle().Foreground(SubtleColor).Render(fmt.Sprintf(" #%d", t.ADOItemID))
	}

	title := prefix + t.Title
	if len(title) > contentWidth-2 {
		title = title[:contentWidth-5] + "..."
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

	var checkboxCounter string
	if t.CheckboxTotal > 0 {
		checkboxCounter = lipgloss.NewStyle().
			Foreground(SubtleColor).
			Render(fmt.Sprintf(" %d/%d", t.CheckboxDone, t.CheckboxTotal))
	}

	box := style.Width(contentWidth).Render(title + adoBadge + tags + checkboxCounter)

	barChar := " "
	barStyle := lipgloss.NewStyle()
	if groupColor != "" {
		barChar = "│"
		barStyle = barStyle.Foreground(groupColor)
	}

	// Span the bar across every line of the box, since tags can wrap a task
	// onto multiple lines.
	height := lipgloss.Height(box)
	barLines := make([]string, height)
	for i := range barLines {
		barLines[i] = barStyle.Render(barChar)
	}
	bar := strings.Join(barLines, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, box, bar)
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
		msgWidth := m.width - 4
		if msgWidth < 20 {
			msgWidth = 20
		}
		parts = append(parts, StatusBarStyle.Foreground(WarningColor).Width(msgWidth).Render(m.message))
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

	if t.ADOItemID > 0 {
		return m.renderADOTaskDetail(t)
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

func (m Model) renderADOTaskDetail(t *task.Task) string {
	availableHeight := m.height - 8

	heading := lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).
		Render(fmt.Sprintf("#%d  %s", t.ADOItemID, t.Title))

	separator := lipgloss.NewStyle().Foreground(SubtleColor).
		Render(strings.Repeat("─", m.width-8))

	var metaParts []string
	metaParts = append(metaParts, "Status: "+string(t.Status))
	if len(t.Tags) > 0 {
		metaParts = append(metaParts, "Tags: "+strings.Join(t.Tags, ", "))
	}
	meta := lipgloss.NewStyle().Foreground(SubtleColor).Render(strings.Join(metaParts, "  ·  "))

	desc := t.Description
	if desc == "" {
		desc = lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("(no description)")
	}

	lines := strings.Split(desc, "\n")
	if m.viewScroll >= len(lines) {
		m.viewScroll = len(lines) - 1
	}
	if m.viewScroll < 0 {
		m.viewScroll = 0
	}
	end := m.viewScroll + availableHeight - 6
	if end > len(lines) {
		end = len(lines)
	}
	visibleDesc := strings.Join(lines[m.viewScroll:end], "\n")

	footer := lipgloss.NewStyle().Foreground(SubtleColor).
		Render("[q: back  H/L: move lane  e: edit title  j/k: scroll]")

	detailStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveColor).
		Padding(1, 2).
		Width(m.width - 4).
		Height(availableHeight + 2)

	return detailStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			heading,
			meta,
			separator,
			visibleDesc,
			separator,
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
  ctrl+u   half page up         H/shift+← move task left
  ctrl+f   half page down       L/shift+→ move task right
                                enter   view task

  MISC                          COMMANDS
  ────                          ────────
  ?        toggle help          :       command mode
  r        refresh              :q      quit
  q        quit                 :b      board switcher
  ctrl+b   board switcher       :pats   PAT manager
                                :logs   error/log viewer

  Press ? or ESC to close help
`
	return DialogStyle.Width(60).Render(help)
}

func (m Model) activeBoardIndex() int {
	for i, b := range m.config.Boards {
		if b.Name == m.config.CurrentBoard {
			return i
		}
	}
	return 0
}

func (m Model) handleBoardSelectorMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	boards := m.config.Boards
	switch msg.String() {
	case "j", "down":
		m.boardSelectorIndex++
		if m.boardSelectorIndex >= len(boards) {
			m.boardSelectorIndex = 0
		}
	case "k", "up":
		m.boardSelectorIndex--
		if m.boardSelectorIndex < 0 {
			m.boardSelectorIndex = len(boards) - 1
		}
	case "enter":
		if m.boardSelectorIndex < len(boards) {
			return m.switchBoard(boards[m.boardSelectorIndex].Name)
		}
	case "a":
		m.mode = ModeInput
		m.inputAction = "new-board-name"
		m.textInput.Reset()
		m.textInput.Placeholder = "Board name..."
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		return m, textinput.Blink
	case "A":
		return m.startADOSetup()
	case "esc", "q":
		m.mode = ModeNormal
	}
	return m, nil
}

func (m Model) createBoard(name, directory string) (tea.Model, tea.Cmd) {
	m.config.AddBoard(name, directory)
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		m.mode = ModeNormal
		return m, nil
	}
	if err := m.config.EnsureTaskDirectory(); err != nil {
		m.message = "Error creating directory: " + err.Error()
		m.logs.Error(m.message)
	}
	return m.switchBoard(name)
}

func (m Model) switchBoard(name string) (tea.Model, tea.Cmd) {
	m.config.CurrentBoard = name
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		m.mode = ModeNormal
		return m, nil
	}
	m.board = task.NewBoardWithColumns(m.config.AllColumns(), m.config.HiddenColumns())
	m.activeLane = 0
	m.activeTask = 0
	m.mode = ModeNormal
	m.lastSyncTime = time.Time{}
	m.minimizedLanes = make(map[task.Status]bool)
	m.message = "Switched to board: " + name
	if m.config.IsADOBoard() {
		m.message = "Switched to board: " + name + " — syncing..."
	}
	return m, m.loadTasks
}

func (m Model) renderBoardSelector() string {
	boards := m.config.Boards
	if len(boards) == 0 {
		return DialogStyle.Render("No boards configured.\nAdd boards to ~/.cmdban.yaml")
	}

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render("  Select Board  "))
	rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render("  j/k: navigate  enter: select  a: new local  A: new ADO  esc: cancel"))
	rows = append(rows, "")

	for i, b := range boards {
		active := b.Name == m.config.CurrentBoard
		cursor := "  "
		if i == m.boardSelectorIndex {
			cursor = "▶ "
		}

		nameStyle := lipgloss.NewStyle()
		if i == m.boardSelectorIndex {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#FFFFFF")).Background(ActiveColor).Bold(true)
		} else if active {
			nameStyle = nameStyle.Foreground(HighlightColor).Bold(true)
		}

		activeMark := ""
		if active {
			activeMark = lipgloss.NewStyle().Foreground(SuccessColor).Render(" ✓")
		}

		adoBadge := ""
		if b.Type == config.BoardTypeAzureDevOps {
			adoBadge = lipgloss.NewStyle().Foreground(HighlightColor).Render(" [ADO]")
		}

		dir := ""
		if b.Type != config.BoardTypeAzureDevOps && b.Directory != "" {
			dir = lipgloss.NewStyle().Foreground(SubtleColor).Render("  " + b.Directory)
		} else if b.Type == config.BoardTypeAzureDevOps && b.AzureDevOps != nil {
			dir = lipgloss.NewStyle().Foreground(SubtleColor).Render(
				fmt.Sprintf("  %s / %s / %s", b.AzureDevOps.Org, b.AzureDevOps.Project, b.AzureDevOps.Team),
			)
		}

		row := cursor + nameStyle.Render(b.Name) + adoBadge + activeMark + "\n" + dir
		rows = append(rows, row)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return DialogStyle.Width(60).Render(content)
}

func (m Model) renderPATManager() string {
	pats := m.patStore.PATs

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render("  PAT Manager  "))
	rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render(
		"  j/k: navigate  a: add  x: delete  space: reveal  esc: close",
	))
	rows = append(rows, "")

	if len(pats) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  No PATs stored yet. Press 'a' to add one."))
	}

	for i, p := range pats {
		cursor := "  "
		if i == m.patSelectorIndex {
			cursor = "▶ "
		}

		nameStyle := lipgloss.NewStyle()
		if i == m.patSelectorIndex {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#FFFFFF")).Background(ActiveColor).Bold(true)
		}

		var tokenDisplay string
		if i == m.revealedPATIndex {
			tokenDisplay = lipgloss.NewStyle().Foreground(WarningColor).Render(p.Token)
		} else {
			masked := maskToken(p.Token)
			tokenDisplay = lipgloss.NewStyle().Foreground(SubtleColor).Render(masked)
		}

		row := cursor + nameStyle.Render(fmt.Sprintf("%-20s", p.Name)) + "  " + tokenDisplay
		rows = append(rows, row)
	}

	rows = append(rows, "")
	rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render(
		fmt.Sprintf("  Stored in %s (0600)", pat.DefaultStorePath()),
	))

	if m.message != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(SuccessColor).Render("  "+m.message))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return DialogStyle.Width(70).Render(content)
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("•", len(token))
	}
	return token[:4] + strings.Repeat("•", 12) + token[len(token)-4:]
}

// bestMatchIndex returns the index of the first case-insensitive match for hint
// in items, falling back to 0 if nothing matches.
func bestMatchIndex(items []string, hint string) int {
	hint = strings.ToLower(hint)
	for i, item := range items {
		if strings.ToLower(item) == hint {
			return i
		}
	}
	// partial match fallback
	for i, item := range items {
		if strings.Contains(strings.ToLower(item), hint) {
			return i
		}
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
