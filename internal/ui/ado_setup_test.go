package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
	"cmdban/internal/pat"
)

func TestParseADOBoardURLBoardURL(t *testing.T) {
	org, project, team, level, isBacklog, err := parseADOBoardURL(
		"https://dev.azure.com/myorg/MyProject/_boards/board/t/MyTeam/Features",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org != "myorg" || project != "MyProject" || team != "MyTeam" || level != "Features" {
		t.Fatalf("got org=%q project=%q team=%q level=%q", org, project, team, level)
	}
	if isBacklog {
		t.Fatal("expected isBacklog = false for a board URL")
	}
}

func TestParseADOBoardURLBacklogURL(t *testing.T) {
	org, project, team, level, isBacklog, err := parseADOBoardURL(
		"https://dev.azure.com/lfantdevelophub/common/_backlogs/backlog/Sejfa%20Internal/Epics",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org != "lfantdevelophub" || project != "common" || team != "Sejfa Internal" || level != "Epics" {
		t.Fatalf("got org=%q project=%q team=%q level=%q", org, project, team, level)
	}
	if !isBacklog {
		t.Fatal("expected isBacklog = true for a backlog URL")
	}
}

func TestParseADOBoardURLBacklogURLNoLevel(t *testing.T) {
	org, project, team, level, isBacklog, err := parseADOBoardURL(
		"https://dev.azure.com/myorg/MyProject/_backlogs/backlog/MyTeam",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org != "myorg" || project != "MyProject" || team != "MyTeam" || level != "" {
		t.Fatalf("got org=%q project=%q team=%q level=%q", org, project, team, level)
	}
	if !isBacklog {
		t.Fatal("expected isBacklog = true for a backlog URL")
	}
}

func TestParseADOBoardURLInvalid(t *testing.T) {
	if _, _, _, _, _, err := parseADOBoardURL("https://example.com/not/an/ado/url"); err == nil {
		t.Fatal("expected error for unrecognised URL")
	}
}

func editableADOBoard() config.Board {
	return config.Board{
		Name: "team-sprint",
		Type: config.BoardTypeAzureDevOps,
		AzureDevOps: &config.AzureDevOpsConfig{
			Org:                 "myorg",
			Project:             "MyProject",
			Team:                "MyTeam",
			Iteration:           `MyProject\Sprint 2`,
			PAT:                 "work",
			AssignedToMe:        true,
			IncludeUnassigned:   true,
			DefaultWorkItemType: "Task",
			LaneMode:            config.LaneModeStates,
			BoardLevel:          "Stories",
			BoardColumnField:    "WEF_Old_Kanban.Column",
			BoardColumns:        []config.BoardColumnMapping{{Lane: "doing", Column: "Doing"}},
		},
		Columns: []config.Column{{Name: "doing", Hidden: true}},
	}
}

func TestADOSetupStateForKanbanBoardPrefillsWizard(t *testing.T) {
	got := adoSetupStateFor(editableADOBoard(), []string{"personal", "work"})

	if got.step != adoSetupStepSelectPAT || got.selectorIndex != 1 || got.patAlias != "work" {
		t.Fatalf("PAT step = %v index %d alias %q, want PAT step preselecting work", got.step, got.selectorIndex, got.patAlias)
	}
	if got.org != "myorg" || got.project != "MyProject" || got.team != "MyTeam" || got.iteration != `MyProject\Sprint 2` {
		t.Fatalf("got org=%q project=%q team=%q iteration=%q", got.org, got.project, got.team, got.iteration)
	}
	if got.isBacklog || got.boardLevel != "Stories" {
		t.Fatalf("isBacklog=%v boardLevel=%q, want Kanban board level Stories", got.isBacklog, got.boardLevel)
	}
	if got.boardName != "team-sprint" {
		t.Fatalf("boardName=%q, want team-sprint", got.boardName)
	}
}

func TestADOSetupStateForBacklogBoardUsesBacklogLevel(t *testing.T) {
	board := editableADOBoard()
	board.AzureDevOps.BacklogLevel = "Epics"

	got := adoSetupStateFor(board, []string{"work"})

	if !got.isBacklog || got.boardLevel != "Epics" {
		t.Fatalf("isBacklog=%v boardLevel=%q, want backlog level Epics", got.isBacklog, got.boardLevel)
	}
}

func TestSuggestBoardNameOnlyForNewBoards(t *testing.T) {
	newSetup := adoSetupState{project: "My Project"}
	newSetup.suggestBoardName()
	if newSetup.boardName != "my-project-sprint" {
		t.Fatalf("new board name = %q, want my-project-sprint", newSetup.boardName)
	}

	edited := adoSetupState{project: "My Project", boardName: "keep-me", editing: &boardEdit{name: "keep-me"}}
	edited.suggestBoardName()
	if edited.boardName != "keep-me" {
		t.Fatalf("edited board name = %q, want keep-me", edited.boardName)
	}
}

func TestIterationSelectorIndexOffsetsPastCurrentIterationOption(t *testing.T) {
	iterations := []azuredevops.Iteration{{Path: `P\Sprint 1`}, {Path: `P\Sprint 2`}}

	cases := map[string]int{`P\Sprint 2`: 2, "@CurrentIteration": 0, "": 0, `P\Gone`: 0}
	for path, want := range cases {
		if got := iterationSelectorIndex(iterations, path); got != want {
			t.Errorf("iterationSelectorIndex(%q) = %d, want %d", path, got, want)
		}
	}
}

func TestADOBoardFromSetupEditKeepsPreferencesAndLaneVisibility(t *testing.T) {
	existing := editableADOBoard()
	setup := adoSetupState{
		org: "neworg", project: "NewProject", team: "NewTeam", iteration: "@CurrentIteration", patAlias: "personal",
		boardDetail: azuredevops.KanbanBoardDetail{
			Name:    "Stories",
			Columns: []azuredevops.BoardColumn{{Name: "Doing"}, {Name: "Review"}},
		},
	}

	got := adoBoardFromSetup(setup, "renamed", &existing)

	ado := got.AzureDevOps
	if got.Name != "renamed" || ado.Org != "neworg" || ado.Project != "NewProject" || ado.Team != "NewTeam" ||
		ado.Iteration != "@CurrentIteration" || ado.PAT != "personal" {
		t.Fatalf("identity not replaced: %+v / %+v", got, ado)
	}
	if !ado.AssignedToMe || !ado.IncludeUnassigned || ado.DefaultWorkItemType != "Task" || ado.LaneMode != config.LaneModeStates {
		t.Fatalf("preferences lost: %+v", ado)
	}
	if len(got.Columns) != 2 || !got.Columns[0].Hidden || got.Columns[1].Hidden {
		t.Fatalf("Columns = %+v, want doing hidden (kept) and review visible", got.Columns)
	}
	if existing.AzureDevOps.Org != "myorg" {
		t.Fatalf("existing board mutated: org = %q", existing.AzureDevOps.Org)
	}
}

func TestADOBoardFromSetupNewBoardDefaultsToUserStory(t *testing.T) {
	got := adoBoardFromSetup(adoSetupState{org: "o", project: "p", team: "t"}, "fresh", nil)

	if got.AzureDevOps.DefaultWorkItemType != azuredevops.WorkItemTypeUserStory {
		t.Fatalf("DefaultWorkItemType = %q, want %q", got.AzureDevOps.DefaultWorkItemType, azuredevops.WorkItemTypeUserStory)
	}
}

func TestADOBoardFromSetupWithoutBoardColumnsClearsStaleMappings(t *testing.T) {
	existing := editableADOBoard()

	got := adoBoardFromSetup(adoSetupState{org: "o", project: "p", team: "t"}, "team-sprint", &existing)

	if got.AzureDevOps.UsesBoardColumns() || got.AzureDevOps.BoardLevel != "" {
		t.Fatalf("board column mapping kept: %+v", got.AzureDevOps)
	}
	if got.AzureDevOps.ColumnMap["done"] != "Closed" || len(got.Columns) != 4 {
		t.Fatalf("default state lanes not applied: %+v / %+v", got.Columns, got.AzureDevOps.ColumnMap)
	}
}

func boardSelectorModel(t *testing.T, boards ...config.Board) Model {
	t.Helper()
	cfg := savableConfig(t, &config.Config{CurrentBoard: boards[0].Name, Boards: boards})
	m := NewModel(cfg)
	m.width, m.height = testWidth, testHeight
	m.patStore = &pat.Store{PATs: []pat.Entry{{Name: "personal"}, {Name: "work"}}}
	m.mode = ModeBoardSelector
	m.boardSelectorIndex = len(boards) - 1
	return m
}

func TestBoardSelectorEditOnADOBoardStartsPrefilledWizard(t *testing.T) {
	m := boardSelectorModel(t, config.Board{Name: "local", Directory: "tasks"}, editableADOBoard())

	m = pressKey(t, m, "e")

	if m.mode != ModeADOSetup || m.adoSetup.step != adoSetupStepSelectPAT {
		t.Fatalf("mode=%v step=%v, want ADO setup at PAT step", m.mode, m.adoSetup.step)
	}
	if edit := m.adoSetup.editing; edit == nil || *edit != (boardEdit{index: 1, name: "team-sprint"}) || m.adoSetup.selectorIndex != 1 {
		t.Fatalf("editing=%+v selectorIndex=%d, want board 1 team-sprint preselecting work", edit, m.adoSetup.selectorIndex)
	}
}

func TestSaveEditedADOBoardReplacesItInPlace(t *testing.T) {
	m := boardSelectorModel(t, config.Board{Name: "local", Directory: "tasks"}, editableADOBoard())
	m = pressKey(t, m, "e")
	m.adoSetup.team = "OtherTeam"

	updated, _ := m.saveADOBoard("renamed")
	got := updated.(Model)

	if len(got.config.Boards) != 2 {
		t.Fatalf("len(Boards) = %d, want 2 (edited, not appended)", len(got.config.Boards))
	}
	if b := got.config.Boards[1]; b.Name != "renamed" || b.AzureDevOps.Team != "OtherTeam" {
		t.Fatalf("Boards[1] = %q team %q, want renamed / OtherTeam", b.Name, b.AzureDevOps.Team)
	}
	if got.config.CurrentBoard != "renamed" {
		t.Fatalf("CurrentBoard = %q, want renamed", got.config.CurrentBoard)
	}
}

func TestBoardSelectorEditOnLocalBoardRenamesAndMovesDirectory(t *testing.T) {
	m := boardSelectorModel(t, editableADOBoard(), config.Board{Name: "local", Directory: "tasks"})
	dir := filepath.Join(t.TempDir(), "moved")
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	m = pressKey(t, m, "e")
	if m.textInput.Value() != "local" {
		t.Fatalf("name input = %q, want prefilled local", m.textInput.Value())
	}
	m.textInput.SetValue("notes")
	updated, _ := m.Update(enter)
	m = updated.(Model)
	if m.textInput.Value() != "tasks" {
		t.Fatalf("directory input = %q, want prefilled tasks", m.textInput.Value())
	}
	m.textInput.SetValue(dir)
	updated, _ = m.Update(enter)
	m = updated.(Model)

	if b := m.config.Boards[1]; b.Name != "notes" || b.Directory != dir {
		t.Fatalf("Boards[1] = %+v, want notes at %s", b, dir)
	}
	if m.config.CurrentBoard != "notes" {
		t.Fatalf("CurrentBoard = %q, want notes", m.config.CurrentBoard)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("board directory not created: %v", err)
	}
}

func TestSaveEditedADOBoardWithDuplicateNameEditsSelectedBoardOnly(t *testing.T) {
	first := editableADOBoard()
	first.AzureDevOps.Team = "Change Requests"
	second := editableADOBoard()
	second.AzureDevOps.Team = "common Team"
	m := boardSelectorModel(t, first, second)

	m = pressKey(t, m, "e")
	m.adoSetup.iteration = `MyProject\\Sprint 3`
	updated, _ := m.saveADOBoard("team-sprint")
	m = updated.(Model)

	if m.mode != ModeInput || m.inputAction != "ado-board-name" || m.inputError == "" {
		t.Fatalf("mode=%v action=%q error=%q, want name prompt reopened with an error", m.mode, m.inputAction, m.inputError)
	}
	if m.config.Boards[0].AzureDevOps.Team != "Change Requests" || m.config.Boards[1].AzureDevOps.Iteration != second.AzureDevOps.Iteration {
		t.Fatal("a board changed although the name was rejected")
	}

	updated, _ = m.saveADOBoard("common-team-sprint")
	m = updated.(Model)

	if got := m.config.Boards[0]; got.Name != "team-sprint" || got.AzureDevOps.Team != "Change Requests" || got.AzureDevOps.Iteration != first.AzureDevOps.Iteration {
		t.Fatalf("Boards[0] = %q %+v, want untouched", got.Name, got.AzureDevOps)
	}
	if got := m.config.Boards[1]; got.Name != "common-team-sprint" || got.AzureDevOps.Team != "common Team" || got.AzureDevOps.Iteration != `MyProject\\Sprint 3` {
		t.Fatalf("Boards[1] = %q %+v, want edited", got.Name, got.AzureDevOps)
	}
	if m.config.CurrentBoard != "common-team-sprint" {
		t.Fatalf("CurrentBoard = %q, want common-team-sprint", m.config.CurrentBoard)
	}
}

func TestNewADOBoardWithTakenNameReopensNamePrompt(t *testing.T) {
	m := boardSelectorModel(t, editableADOBoard())
	m.adoSetup = adoSetupState{org: "o", project: "p", team: "t"}

	updated, _ := m.saveADOBoard("team-sprint")
	m = updated.(Model)

	if len(m.config.Boards) != 1 {
		t.Fatalf("len(Boards) = %d, want 1 (duplicate not added)", len(m.config.Boards))
	}
	if m.inputAction != "ado-board-name" || m.textInput.Value() != "team-sprint" || !strings.Contains(m.View(), "already named") {
		t.Fatalf("action=%q value=%q, want name prompt showing the taken-name error", m.inputAction, m.textInput.Value())
	}
}

func TestNewLocalBoardWithTakenNameStaysOnNamePrompt(t *testing.T) {
	m := boardSelectorModel(t, config.Board{Name: "local", Directory: "tasks"})

	m = pressKey(t, m, "a")
	m.textInput.SetValue("local")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.inputAction != "new-board-name" || m.inputError == "" {
		t.Fatalf("action=%q error=%q, want name prompt with an error", m.inputAction, m.inputError)
	}
}

func TestBoardSelectorRenameRenamesOnlyAndStaysInSwitcher(t *testing.T) {
	ado := editableADOBoard()
	m := boardSelectorModel(t, config.Board{Name: "local", Directory: "tasks"}, ado)
	m.config.RecentBoards = []string{"team-sprint", "local"}

	m = pressKey(t, m, "r")
	if m.textInput.Value() != "team-sprint" {
		t.Fatalf("name input = %q, want prefilled team-sprint", m.textInput.Value())
	}
	m.textInput.SetValue("renamed")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.mode != ModeBoardSelector {
		t.Fatalf("mode = %v, want board switcher", m.mode)
	}
	got := m.config.Boards[1]
	if got.Name != "renamed" || got.AzureDevOps.Team != ado.AzureDevOps.Team || len(got.AzureDevOps.BoardColumns) != 1 {
		t.Fatalf("Boards[1] = %q %+v, want only the name changed", got.Name, got.AzureDevOps)
	}
	if m.config.CurrentBoard != "local" || m.config.RecentBoards[0] != "renamed" {
		t.Fatalf("CurrentBoard = %q recent = %v, want local still active and recent list renamed", m.config.CurrentBoard, m.config.RecentBoards)
	}
	reloaded, err := config.Load()
	if err != nil || reloaded.Boards[1].Name != "renamed" {
		t.Fatalf("rename not persisted (err = %v)", err)
	}
}

func TestBoardSelectorRenameRejectsTakenName(t *testing.T) {
	m := boardSelectorModel(t, config.Board{Name: "local", Directory: "tasks"}, editableADOBoard())

	m = pressKey(t, m, "r")
	m.textInput.SetValue("local")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.inputAction != inputActionRenameBoard || m.inputError == "" || m.config.Boards[1].Name != "team-sprint" {
		t.Fatalf("action=%q error=%q name=%q, want prompt kept open and board unchanged", m.inputAction, m.inputError, m.config.Boards[1].Name)
	}
}
