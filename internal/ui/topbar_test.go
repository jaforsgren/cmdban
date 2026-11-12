package ui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

const testWidth, testHeight = 170, 40

func adoTestConfig() *config.Config {
	return &config.Config{
		CurrentBoard: "sprint",
		Boards: []config.Board{
			{Name: "local", Directory: "/tmp/local", Columns: []config.Column{{Name: "today"}}},
			{
				Name: "sprint",
				Type: config.BoardTypeAzureDevOps,
				AzureDevOps: &config.AzureDevOpsConfig{
					Org: "org", Project: "proj", Team: "team",
					BoardLevel:       "Stories",
					BoardColumnField: "WEF_X_Kanban.Column",
					BoardColumns: []config.BoardColumnMapping{
						{Lane: "todo", Column: "ToDo", States: map[string]string{"Bug": "New"}},
						{Lane: "review", Column: "Review", States: map[string]string{"Bug": "Active"}},
					},
				},
				Columns: []config.Column{{Name: "todo"}, {Name: "review"}},
			},
		},
	}
}

// savableConfig returns a config backed by a file in a temp HOME, so model
// actions that persist the config succeed.
func savableConfig(t *testing.T, cfg *config.Config) *config.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	loaded.CurrentBoard = cfg.CurrentBoard
	loaded.Boards = cfg.Boards
	loaded.RecentBoards = cfg.RecentBoards
	return loaded
}

func adoTestModel(cfg *config.Config) Model {
	m := NewModel(cfg)
	m.width, m.height = testWidth, testHeight
	m.board.AddTask(&task.Task{ID: "1", Title: "Bug one", Status: "todo", ADOWorkItemType: "Bug", ADOItemID: 1})
	m.board.AddTask(&task.Task{ID: "2", Title: "Story one", Status: "review", ADOWorkItemType: "User Story", ADOItemID: 2})
	m.board.AddTask(&task.Task{ID: "3", Title: "Task one", Status: "review", ADOWorkItemType: "Task", ADOItemID: 3})
	m.board.AddTask(&task.Task{ID: "4", Title: "Odd one", Status: "todo", ADOWorkItemType: "Risk", ADOItemID: 4})
	return m
}

func pressKey(t *testing.T, m Model, k string) Model {
	t.Helper()
	got, _ := m.Update(keyMsgFor(k))
	return got.(Model)
}

func TestWorkItemTypesUsePreferredOrderThenAlphabetical(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	want := []string{"User Story", "Bug", "Task", "Risk"}
	if got := m.workItemTypes(); !slices.Equal(got, want) {
		t.Fatalf("workItemTypes() = %v, want %v", got, want)
	}
}

func shownTitles(m Model) []string {
	var titles []string
	for _, lane := range m.board.Lanes {
		for _, t := range m.filterTasks(m.board.Tasks[lane]) {
			titles = append(titles, t.Title)
		}
	}
	slices.Sort(titles)
	return titles
}

func TestDefaultTypeViewShowsStoriesAndBugs(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	if got, want := shownTitles(m), []string{"Bug one", "Story one"}; !slices.Equal(got, want) {
		t.Fatalf("shown = %v, want %v", got, want)
	}
	if m.hasActiveFilter() {
		t.Fatal("hasActiveFilter() = true for the default view, want false")
	}
}

func TestDefaultTypeViewShowsEverythingWithoutStoriesOrBugs(t *testing.T) {
	m := NewModel(adoTestConfig())
	m.board.AddTask(&task.Task{ID: "1", Title: "Epic one", Status: "todo", ADOWorkItemType: "Epic", ADOItemID: 1})

	if got := shownTitles(m); !slices.Equal(got, []string{"Epic one"}) {
		t.Fatalf("shown = %v, want the epic when there are no stories or bugs", got)
	}
}

func TestTypeFilterKeysSelectSingleAllAndDefault(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "3")
	if got := shownTitles(m); !slices.Equal(got, []string{"Task one"}) {
		t.Fatalf("key 3 shown = %v, want only the Task", got)
	}

	m = pressKey(t, m, "9")
	if got := shownTitles(m); len(got) != 4 {
		t.Fatalf("key 9 shown = %v, want all 4 tasks", got)
	}

	m = pressKey(t, m, "0")
	if m.typeFilter.kind != typeFilterDefault {
		t.Fatalf("typeFilter = %+v after 0, want the default view", m.typeFilter)
	}
}

func TestTypeFilterKeyBeyondTypesShowsMessage(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "8")

	if m.typeFilter.kind != typeFilterDefault || m.message == "" {
		t.Fatalf("typeFilter = %+v, message = %q; want the default view and an explanation", m.typeFilter, m.message)
	}
}

func TestTypeFilterKeyOnLocalBoardShowsMessage(t *testing.T) {
	cfg := adoTestConfig()
	cfg.CurrentBoard = "local"
	m := pressKey(t, adoTestModel(cfg), "1")

	if m.typeFilter.kind != typeFilterDefault || m.message == "" {
		t.Fatalf("typeFilter = %+v, message = %q; want no filter and an explanation", m.typeFilter, m.message)
	}
}

func TestEscapeClearsTypeAndSearchFilters(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.typeFilter = typeFilter{kind: typeFilterSingle, workItemType: "Bug"}
	m.searchFilter = "one"

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = got.(Model)

	if m.hasActiveFilter() {
		t.Fatalf("filters = (%+v, %q), want both cleared", m.typeFilter, m.searchFilter)
	}
}

func TestCardsShowTypeGlyphOnlyForADOItems(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	bug := m.renderTask(m.board.Tasks["todo"][0], false, 40, "")
	if !strings.Contains(bug, typeGlyphs["Bug"].symbol) {
		t.Fatalf("bug card missing its glyph: %q", bug)
	}
	local := m.renderTask(&task.Task{Title: "local"}, false, 40, "")
	if strings.Contains(local, fallbackTypeGlyph.symbol) {
		t.Fatalf("local card should have no type glyph: %q", local)
	}
	if lipgloss.Width(bug) != lipgloss.Width(local) {
		t.Fatalf("card widths differ: bug %d, local %d", lipgloss.Width(bug), lipgloss.Width(local))
	}
}

func TestToggleLaneModeSwitchesToStateLanes(t *testing.T) {
	m := adoTestModel(savableConfig(t, adoTestConfig()))

	m = pressKey(t, m, "v")

	want := []task.Status{"new", "active"}
	if !slices.Equal(m.board.Lanes, want) {
		t.Fatalf("Lanes = %v, want %v", m.board.Lanes, want)
	}
}

func TestToggleLaneModeWithoutBoardColumnsShowsMessage(t *testing.T) {
	cfg := adoTestConfig()
	cfg.Boards[1].AzureDevOps.BoardColumnField = ""
	m := pressKey(t, adoTestModel(cfg), "v")

	if m.message == "" {
		t.Fatal("message = empty, want a hint to run :adocolumns")
	}
}

func TestTopBarKeepsFixedHeightAndShowsSections(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	bar := m.renderTopBar()

	if got := lipgloss.Height(bar); got != topBarHeight {
		t.Fatalf("topbar height = %d, want %d", got, topBarHeight)
	}
	for _, want := range []string{"CONTEXT", "RECENT", "TYPES", "SHORTCUTS", "<1>", "User Story", "Stories"} {
		if !strings.Contains(bar, want) {
			t.Errorf("topbar missing %q:\n%s", want, bar)
		}
	}
}

func TestTopBarOmitsTypesOnLocalBoard(t *testing.T) {
	cfg := adoTestConfig()
	cfg.CurrentBoard = "local"
	m := adoTestModel(cfg)

	if bar := m.renderTopBar(); strings.Contains(bar, "TYPES") {
		t.Fatalf("local board topbar should not show TYPES:\n%s", bar)
	}
}

func TestViewFillsTerminalHeight(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	if got := lipgloss.Height(m.View()); got != testHeight {
		t.Fatalf("View() height = %d, want %d", got, testHeight)
	}
}

func TestContextualShortcutsIncludeADOKeysOnlyOnADOBoards(t *testing.T) {
	hasTypeFilter := func(shortcuts []shortcut) bool {
		return slices.ContainsFunc(shortcuts, func(s shortcut) bool { return s.key == "0/1-8/9" })
	}

	if !hasTypeFilter(adoTestModel(adoTestConfig()).contextualShortcuts()) {
		t.Fatal("ADO board shortcuts should include the type filter")
	}
	cfg := adoTestConfig()
	cfg.CurrentBoard = "local"
	if hasTypeFilter(adoTestModel(cfg).contextualShortcuts()) {
		t.Fatal("local board shortcuts should not include the type filter")
	}
}

func TestHelpListsShortcutsAndCommandsAndScrollIsClamped(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "?")
	if m.mode != ModeHelp {
		t.Fatalf("mode = %v, want ModeHelp", m.mode)
	}

	help := m.renderHelp()
	for _, want := range []string{"Board", "Commands", ":adocolumns", "ctrl+n"} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing %q", want)
		}
	}

	m = pressKey(t, m, "G")
	if m.viewScroll != m.maxHelpScroll() {
		t.Fatalf("viewScroll = %d after G, want %d", m.viewScroll, m.maxHelpScroll())
	}
	m = pressKey(t, m, "j")
	if m.viewScroll != m.maxHelpScroll() {
		t.Fatalf("viewScroll = %d after j at bottom, want clamped to %d", m.viewScroll, m.maxHelpScroll())
	}

	m = pressKey(t, m, "?")
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v after ?, want ModeNormal", m.mode)
	}
}

func TestCycleRecentBoardKeepsRecentOrder(t *testing.T) {
	cfg := adoTestConfig()
	cfg.RecentBoards = []string{"sprint", "local"}
	m := adoTestModel(savableConfig(t, cfg))

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = got.(Model)

	if m.config.CurrentBoard != "local" {
		t.Fatalf("CurrentBoard = %q, want %q", m.config.CurrentBoard, "local")
	}
	if want := []string{"sprint", "local"}; !slices.Equal(m.config.RecentBoards, want) {
		t.Fatalf("RecentBoards = %v, want unchanged %v", m.config.RecentBoards, want)
	}
}

func TestSwitchBoardMovesBoardToFrontOfRecent(t *testing.T) {
	cfg := adoTestConfig()
	cfg.RecentBoards = []string{"sprint"}
	m := adoTestModel(savableConfig(t, cfg))

	got, _ := m.switchBoard("local")
	m = got.(Model)

	if want := []string{"local", "sprint"}; !slices.Equal(m.config.RecentBoards, want) {
		t.Fatalf("RecentBoards = %v, want %v", m.config.RecentBoards, want)
	}
}

func TestNewTaskLaneOnADOBoardIsFirstColumn(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.activeLane = 1

	if got := m.newTaskLane(); got != "todo" {
		t.Fatalf("newTaskLane() = %q, want %q", got, "todo")
	}
}

func TestNewTaskLaneOnLocalBoardIsActiveLane(t *testing.T) {
	m := NewModel(&config.Config{})
	m.activeLane = 1

	if got := m.newTaskLane(); got != m.board.Lanes[1] {
		t.Fatalf("newTaskLane() = %q, want active lane %q", got, m.board.Lanes[1])
	}
}

func TestDoneToggleTargetOnADOBoardUsesOuterColumns(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	if got := m.doneToggleTarget(&task.Task{Status: "todo"}); got != "review" {
		t.Fatalf("doneToggleTarget(todo) = %q, want last column %q", got, "review")
	}
	if got := m.doneToggleTarget(&task.Task{Status: "review"}); got != "todo" {
		t.Fatalf("doneToggleTarget(review) = %q, want first column %q", got, "todo")
	}
}

func TestADOMoveFailureReportsAndReloads(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	got, cmd := m.Update(adoMoveFailedMsg{err: errBoom})

	if cmd == nil {
		t.Fatal("move failure returned no command, want a reload")
	}
	if msg := got.(Model).message; !strings.Contains(msg, "boom") {
		t.Fatalf("message = %q, want the failure reason", msg)
	}
}

func TestMovingADOTaskRightMovesLocallyAndPushes(t *testing.T) {
	m := adoTestModel(adoTestConfig())

	got, cmd := m.Update(keyMsgFor("L"))
	m = got.(Model)

	if cmd == nil {
		t.Fatal("moving an ADO task returned no command, want the change pushed")
	}
	if m.board.Lanes[m.activeLane] != "review" || m.selectedTask() == nil {
		t.Fatalf("active lane = %q, want the task followed into %q", m.board.Lanes[m.activeLane], "review")
	}
}

func TestBugKeyOnADOBoardPromptsForBugTitle(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "b")

	if m.mode != ModeInput || m.inputAction != inputActionNewBug {
		t.Fatalf("mode = %v, inputAction = %q; want bug title input", m.mode, m.inputAction)
	}
}

func TestBugKeyOnLocalBoardShowsMessage(t *testing.T) {
	cfg := adoTestConfig()
	cfg.CurrentBoard = "local"
	m := pressKey(t, adoTestModel(cfg), "b")

	if m.mode != ModeNormal || m.message == "" {
		t.Fatalf("mode = %v, message = %q; want to stay put with an explanation", m.mode, m.message)
	}
}

func TestSubmittingBugTitleCreatesBug(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "b")
	m.textInput.SetValue("Crash on save")

	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = got.(Model)

	if cmd == nil || m.mode != ModeNormal {
		t.Fatalf("cmd = %v, mode = %v; want the bug created", cmd, m.mode)
	}
	if !strings.Contains(m.message, "Bug") {
		t.Fatalf("message = %q, want it to mention the Bug being created", m.message)
	}
}

func TestDefaultWorkItemTypeOnADOBoard(t *testing.T) {
	cfg := adoTestConfig()
	m := adoTestModel(cfg)
	if got := m.defaultWorkItemType(); got != "User Story" {
		t.Fatalf("defaultWorkItemType() = %q, want %q", got, "User Story")
	}

	cfg.Boards[1].AzureDevOps.DefaultWorkItemType = "Task"
	if got := m.defaultWorkItemType(); got != "Task" {
		t.Fatalf("defaultWorkItemType() = %q, want configured %q", got, "Task")
	}
}
