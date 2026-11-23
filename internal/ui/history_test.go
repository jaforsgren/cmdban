package ui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

const (
	historyTestBoard = "local"
	historyTestTitle = "Original"
)

func newHistoryTestModel(t *testing.T) (Model, *task.Task) {
	t.Helper()
	cfg := &config.Config{
		CurrentBoard: historyTestBoard,
		Boards:       []config.Board{{Name: historyTestBoard, Directory: t.TempDir()}},
	}
	m := NewModel(cfg)
	created, err := m.taskService().Create(historyTestTitle, "", m.board.Lanes[0])
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	m.board.AddTask(created)
	return m, created
}

func pressHistoryKey(t *testing.T, m Model, msg tea.KeyMsg) Model {
	t.Helper()
	updated, cmd := m.Update(msg)
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a command")
	}
	updated, _ = m.Update(cmd())
	return updated.(Model)
}

func TestUndoKeyRevertsAndCtrlRReapplies(t *testing.T) {
	m, created := newHistoryTestModel(t)
	if err := m.taskService().UpdateTitle(created, "Renamed"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}

	m = pressHistoryKey(t, m, keyMsgFor(undoKey))
	parsed, err := task.ParseMarkdownFile(created.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	if parsed.Title != historyTestTitle {
		t.Errorf("after undo Title = %q, want %q", parsed.Title, historyTestTitle)
	}

	m = pressHistoryKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})
	parsed, err = task.ParseMarkdownFile(created.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	if parsed.Title != "Renamed" {
		t.Errorf("after redo Title = %q, want Renamed", parsed.Title)
	}
	if m.message == "" {
		t.Error("expected a status message")
	}
}

func TestUndoWithEmptyHistoryReportsIt(t *testing.T) {
	m := NewModel(&config.Config{CurrentBoard: historyTestBoard})

	m = pressHistoryKey(t, m, keyMsgFor(undoKey))

	if m.message != nothingToUndoMessage {
		t.Errorf("message = %q", m.message)
	}
}

func TestEditorChangeIsUndoable(t *testing.T) {
	m, created := newHistoryTestModel(t)
	before, err := os.ReadFile(created.FilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(created.FilePath, []byte("# Edited\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	updated, _ := m.Update(editorFinishedMsg{filePath: created.FilePath, contentBefore: before})
	m = updated.(Model)

	pressHistoryKey(t, m, keyMsgFor(undoKey))

	restored, err := os.ReadFile(created.FilePath)
	if err != nil || string(restored) != string(before) {
		t.Errorf("content = %q, err = %v", restored, err)
	}
}
