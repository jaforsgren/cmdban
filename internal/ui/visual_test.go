package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

const visualTestBoard = "local"

func newVisualModel(t *testing.T, titles ...string) (Model, []*task.Task) {
	t.Helper()
	cfg := &config.Config{
		CurrentBoard: visualTestBoard,
		Boards:       []config.Board{{Name: visualTestBoard, Directory: t.TempDir()}},
	}
	m := NewModel(cfg)
	m.width, m.height = 120, 40
	var tasks []*task.Task
	for _, title := range titles {
		created, err := m.taskService().Create(title, "", m.board.Lanes[0])
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		m.board.AddTask(created)
		tasks = append(tasks, created)
	}
	return m, tasks
}

func sendVisualKey(t *testing.T, m Model, name string) Model {
	t.Helper()
	updated, _ := m.Update(keyMsgFor(name))
	return updated.(Model)
}

func submitVisualInput(t *testing.T, m Model, value string) (Model, tea.Cmd) {
	t.Helper()
	m.textInput.SetValue(value)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return updated.(Model), cmd
}

func parsedVisualTask(t *testing.T, created *task.Task) *task.Task {
	t.Helper()
	parsed, err := task.ParseMarkdownFile(created.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	return parsed
}

func TestVisualRangeFollowsCursor(t *testing.T) {
	m, tasks := newVisualModel(t, "One", "Two", "Three")

	m = sendVisualKey(t, m, visualModeKey)
	m = sendVisualKey(t, m, visualDownKey)

	targets := m.bulkTargets()
	if len(targets) != 2 || targets[0] != tasks[0] || targets[1] != tasks[1] {
		t.Errorf("targets = %v", taskIDs(targets))
	}
}

func TestVisualTargetsIncludeMarkedCards(t *testing.T) {
	m, tasks := newVisualModel(t, "One", "Two", "Three")
	tasks[2].Marked = true

	m = sendVisualKey(t, m, visualModeKey)

	if targets := m.bulkTargets(); len(targets) != 2 || targets[1] != tasks[2] {
		t.Errorf("targets = %v", taskIDs(targets))
	}
}

func TestVisualMoveAppliesAndUndoesAsOneStep(t *testing.T) {
	m, tasks := newVisualModel(t, "One", "Two")
	targetLane := m.board.Lanes[1]

	m = sendVisualKey(t, m, visualModeKey)
	m = sendVisualKey(t, m, visualDownKey)
	m = sendVisualKey(t, m, visualMoveKey)
	m, cmd := submitVisualInput(t, m, strings.ToUpper(string(targetLane)))
	if cmd == nil {
		t.Fatal("expected a bulk command")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)

	if m.mode != ModeNormal || m.visual.active {
		t.Errorf("mode = %v, visual = %v", m.mode, m.visual.active)
	}
	for _, created := range tasks {
		if got := parsedVisualTask(t, created).Status; got != targetLane {
			t.Errorf("Status = %q, want %q", got, targetLane)
		}
	}

	if _, err := m.taskService().History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for _, created := range tasks {
		if got := parsedVisualTask(t, created).Status; got == targetLane {
			t.Errorf("Status still %q after undo", got)
		}
	}
}

func TestVisualMoveRejectsUnknownLane(t *testing.T) {
	m, _ := newVisualModel(t, "One")

	m = sendVisualKey(t, m, visualModeKey)
	m = sendVisualKey(t, m, visualMoveKey)
	m, cmd := submitVisualInput(t, m, "no-such-lane")

	if cmd != nil || m.mode != ModeInput || m.inputError == "" {
		t.Errorf("mode = %v, inputError = %q, cmd nil = %v", m.mode, m.inputError, cmd == nil)
	}
}

func TestVisualPriorityRejectsNegative(t *testing.T) {
	m, _ := newVisualModel(t, "One")

	m = sendVisualKey(t, m, visualModeKey)
	m = sendVisualKey(t, m, visualPriorityKey)
	m, cmd := submitVisualInput(t, m, "-1")

	if cmd != nil || m.inputError == "" {
		t.Errorf("inputError = %q, cmd nil = %v", m.inputError, cmd == nil)
	}
}

func TestVisualArchiveAsksFirst(t *testing.T) {
	m, tasks := newVisualModel(t, "One")

	m = sendVisualKey(t, m, visualModeKey)
	m = sendVisualKey(t, m, visualArchiveKey)
	if m.mode != ModeConfirm {
		t.Fatalf("mode = %v, want confirm", m.mode)
	}

	updated, cmd := m.Update(keyMsgFor("n"))
	m = updated.(Model)
	if cmd != nil || m.mode != ModeVisual {
		t.Errorf("after n: mode = %v, cmd nil = %v", m.mode, cmd == nil)
	}
	if parsedVisualTask(t, tasks[0]).Title != "One" {
		t.Error("task changed after declining")
	}
}

func TestVisualEscapeClearsSelection(t *testing.T) {
	m, _ := newVisualModel(t, "One")

	m = sendVisualKey(t, m, visualModeKey)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.mode != ModeNormal || m.visual.active {
		t.Errorf("mode = %v, visual = %v", m.mode, m.visual.active)
	}
}

func TestVisualRangeClampsWhenLaneShrinks(t *testing.T) {
	m, tasks := newVisualModel(t, "One", "Two", "Three")
	m = sendVisualKey(t, m, visualModeKey)
	m.visual.anchor = 2
	m.board.RemoveTask(tasks[2])
	m.board.RemoveTask(tasks[1])
	m.clampSelection()

	if targets := m.bulkTargets(); len(targets) != 1 || targets[0] != tasks[0] {
		t.Errorf("targets = %v", taskIDs(targets))
	}
}
