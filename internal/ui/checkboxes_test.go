package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/checkboxdraft"
	"cmdban/internal/task"
)

func checkboxTestModel(t *testing.T) (Model, *task.Task) {
	t.Helper()
	m := adoTestModel(adoTestConfig())
	m.checkboxDrafts = checkboxdraft.New(t.TempDir() + "/drafts.yaml")
	card := m.selectedTask()
	card.Description = "- [ ] one\n- [ ] two"
	card.RefreshCheckboxCounts()
	return m, card
}

func TestCheckboxChordStagesADOChangeWithoutACommand(t *testing.T) {
	m, card := checkboxTestModel(t)

	m = pressKey(t, m, checkboxPrefixKey)
	updated, cmd := m.handleConfirmMode(keyMsgFor("2"))
	m = updated.(Model)

	if cmd != nil {
		t.Fatal("toggle returned a command, want nothing sent to ADO")
	}
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want the board", m.mode)
	}
	if card.Description != "- [ ] one\n- [x] two" {
		t.Fatalf("description = %q", card.Description)
	}
	if got := m.unsavedCheckboxCount(card); got != 1 {
		t.Fatalf("unsaved = %d, want 1", got)
	}
}

func TestSyncKeepsUnsavedCheckboxChange(t *testing.T) {
	m, card := checkboxTestModel(t)
	updated, _ := m.toggleCheckbox(1)
	m = updated.(Model)

	synced := *card
	synced.Description = "- [ ] one\n- [ ] two"
	synced.RefreshCheckboxCounts()
	m.applyCheckboxDrafts([]*task.Task{&synced})

	if synced.Description != "- [x] one\n- [ ] two" {
		t.Fatalf("description = %q", synced.Description)
	}
	if got := m.unsavedCheckboxCount(&synced); got != 1 {
		t.Fatalf("unsaved = %d, want 1", got)
	}
}

func TestSyncDropsChangeADOAlreadyHas(t *testing.T) {
	m, card := checkboxTestModel(t)
	updated, _ := m.toggleCheckbox(1)
	m = updated.(Model)

	synced := *card
	m.applyCheckboxDrafts([]*task.Task{&synced})

	if got := m.unsavedCheckboxCount(&synced); got != 0 {
		t.Fatalf("unsaved = %d, want 0", got)
	}
}

func TestDeleteAnswerStillWorksFromCheckboxPrompt(t *testing.T) {
	m, _ := checkboxTestModel(t)
	m = pressKey(t, m, checkboxPrefixKey)

	updated, _ := m.handleConfirmMode(keyMsgFor("n"))

	if got := updated.(Model).mode; got != ModeNormal {
		t.Fatalf("mode = %v, want the board", got)
	}
}

func TestDeletePromptListsCheckboxChords(t *testing.T) {
	m, _ := checkboxTestModel(t)
	updated, _ := m.toggleCheckbox(1)
	m = updated.(Model)

	prompt := m.deleteConfirmPrompt()

	for _, want := range []string{"1-2: toggle", "u: discard 1"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt %q missing %q", prompt, want)
		}
	}
}

func TestADODetailShowsAuthor(t *testing.T) {
	m, card := checkboxTestModel(t)
	m.width, m.height = testWidth, 40
	card.ADOAuthor = "Ada Author"

	if got := m.renderADOTaskDetailPane(card, testWidth, ""); !strings.Contains(got, "Author: Ada Author") {
		t.Fatalf("detail pane has no author:\n%s", got)
	}
}

func TestCheckboxChordStaysInTaskView(t *testing.T) {
	m, card := checkboxTestModel(t)
	m.mode = ModeView

	m = pressKey(t, m, checkboxPrefixKey)
	if got := m.displayMode(); got != ModeView {
		t.Fatalf("display mode during prompt = %v, want task view", got)
	}
	m = pressKey(t, m, "1")

	if m.mode != ModeView {
		t.Fatalf("mode = %v, want task view", m.mode)
	}
	if card.CheckboxDone != 1 {
		t.Fatalf("done = %d, want 1", card.CheckboxDone)
	}
}

func TestCancelledDeleteReturnsToTaskView(t *testing.T) {
	m, _ := checkboxTestModel(t)
	m.mode = ModeView

	m = pressKey(t, pressKey(t, m, checkboxPrefixKey), "n")

	if m.mode != ModeView {
		t.Fatalf("mode = %v, want task view", m.mode)
	}
}

func TestCtrlSStartsSaveFromEveryCardScreen(t *testing.T) {
	for _, mode := range []Mode{ModeNormal, ModeView, ModeFocus} {
		m, _ := checkboxTestModel(t)
		updated, _ := m.toggleCheckbox(1)
		m = updated.(Model)
		m.mode = mode

		got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

		if cmd == nil {
			t.Errorf("mode %v: ctrl+s returned no save command", mode)
		}
		if got.(Model).mode != mode {
			t.Errorf("mode = %v, want %v", got.(Model).mode, mode)
		}
	}
}

func TestSKeyNoLongerSavesFromDeletePrompt(t *testing.T) {
	m, _ := checkboxTestModel(t)
	updated, _ := m.toggleCheckbox(1)
	m = pressKey(t, updated.(Model), checkboxPrefixKey)

	got, cmd := m.Update(keyMsgFor("s"))

	if cmd != nil || got.(Model).mode != ModeConfirm {
		t.Fatalf("s in the delete prompt: cmd set = %v, mode = %v", cmd != nil, got.(Model).mode)
	}
}

func TestTopBarLeadsWithCheckboxSaveOnADOScreens(t *testing.T) {
	for _, mode := range []Mode{ModeNormal, ModeView, ModeFocus} {
		m, _ := checkboxTestModel(t)
		m.mode = mode

		if got := m.contextualShortcuts()[0].key; got != checkboxSaveKey {
			t.Errorf("mode %v: first shortcut = %q, want %q", mode, got, checkboxSaveKey)
		}
		if !strings.Contains(strings.Join(m.buildShortcutsDisplay(testWidth), "\n"), checkboxSaveKey) {
			t.Errorf("mode %v: topbar does not show %s", mode, checkboxSaveKey)
		}
	}
}

func TestHelpListsCheckboxSave(t *testing.T) {
	m, _ := checkboxTestModel(t)

	if !strings.Contains(strings.Join(m.helpContentLines(), "\n"), checkboxSaveKey) {
		t.Fatalf("help does not list %s", checkboxSaveKey)
	}
}
