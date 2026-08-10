package ui

import (
	"errors"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/applog"
	"cmdban/internal/config"
	"cmdban/internal/task"
)

var errBoom = errors.New("boom")

func keyMsgFor(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestTaskURLLocalBoardFindsFirstLink(t *testing.T) {
	m := NewModel(&config.Config{})
	tk := &task.Task{
		Description: "see notes\nmore info at https://example.com/doc and http://other.example/page",
	}

	got, ok := m.taskURL(tk)
	if !ok {
		t.Fatalf("taskURL() ok = false, want true")
	}
	if want := "https://example.com/doc"; got != want {
		t.Fatalf("taskURL() = %q, want %q", got, want)
	}
}

func TestTaskURLLocalBoardNoLink(t *testing.T) {
	m := NewModel(&config.Config{})
	tk := &task.Task{Description: "no links here"}

	if _, ok := m.taskURL(tk); ok {
		t.Fatalf("taskURL() ok = true, want false")
	}
}

func TestTaskURLADOBoardConstructsWorkItemLink(t *testing.T) {
	cfg := &config.Config{
		CurrentBoard: "ado",
		Boards: []config.Board{
			{
				Name: "ado",
				Type: config.BoardTypeAzureDevOps,
				AzureDevOps: &config.AzureDevOpsConfig{
					Org:     "lfantdevelophub",
					Project: "common",
				},
			},
		},
	}
	m := NewModel(cfg)
	tk := &task.Task{ADOItemID: 1237}

	got, ok := m.taskURL(tk)
	if !ok {
		t.Fatalf("taskURL() ok = false, want true")
	}
	if want := "https://dev.azure.com/lfantdevelophub/common/_workitems/edit/1237"; got != want {
		t.Fatalf("taskURL() = %q, want %q", got, want)
	}
}

func TestTaskURLADOBoardNoItemID(t *testing.T) {
	cfg := &config.Config{
		CurrentBoard: "ado",
		Boards: []config.Board{
			{
				Name:        "ado",
				Type:        config.BoardTypeAzureDevOps,
				AzureDevOps: &config.AzureDevOpsConfig{Org: "org", Project: "proj"},
			},
		},
	}
	m := NewModel(cfg)
	tk := &task.Task{}

	if _, ok := m.taskURL(tk); ok {
		t.Fatalf("taskURL() ok = true, want false")
	}
}

func TestOpenCommentsLocalBoardShowsMessage(t *testing.T) {
	m := NewModel(&config.Config{})
	m.board.AddTask(&task.Task{ID: "1", Title: "local task", Status: task.StatusToday})

	got, cmd := m.openComments()
	updated := got.(Model)

	if updated.mode == ModeComments {
		t.Fatalf("mode = ModeComments, want unchanged for a local board")
	}
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil for a local board", cmd)
	}
	if updated.message == "" {
		t.Fatalf("message = %q, want a explanation for a local board", updated.message)
	}
}

func TestOpenCommentsADOBoardStartsLoading(t *testing.T) {
	cfg := &config.Config{
		CurrentBoard: "ado",
		Boards: []config.Board{
			{
				Name:        "ado",
				Type:        config.BoardTypeAzureDevOps,
				AzureDevOps: &config.AzureDevOpsConfig{Org: "org", Project: "proj", PAT: "missing"},
			},
		},
	}
	m := NewModel(cfg)
	m.board.AddTask(&task.Task{ID: "1", Title: "ado task", Status: task.StatusToday, ADOItemID: 42})

	got, cmd := m.openComments()
	updated := got.(Model)

	if updated.mode != ModeComments {
		t.Fatalf("mode = %v, want ModeComments", updated.mode)
	}
	if !updated.commentsLoading {
		t.Fatalf("commentsLoading = false, want true")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want a fetch command")
	}

	// The configured PAT isn't stored, so the fetch command should resolve
	// to an error message rather than panic or hang.
	msg := cmd()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("cmd() = %T, want errMsg", msg)
	}
}

func TestADOEditTempFileRoundTrip(t *testing.T) {
	orig := &task.Task{Title: "Fix login bug", Description: "Steps:\n1. do a\n2. do b", ADOItemID: 99}

	path, err := writeADOEditTempFile(orig)
	if err != nil {
		t.Fatalf("writeADOEditTempFile() error = %v", err)
	}
	defer os.Remove(path)

	title, description, err := readADOEditTempFile(path)
	if err != nil {
		t.Fatalf("readADOEditTempFile() error = %v", err)
	}
	if title != orig.Title {
		t.Fatalf("title = %q, want %q", title, orig.Title)
	}
	if description != orig.Description {
		t.Fatalf("description = %q, want %q", description, orig.Description)
	}
}

func TestHandleADOEditorFinishedNoChangeIsNoop(t *testing.T) {
	m := NewModel(&config.Config{})
	orig := &task.Task{Title: "Same", Description: "unchanged", ADOItemID: 1}

	path, err := writeADOEditTempFile(orig)
	if err != nil {
		t.Fatalf("writeADOEditTempFile() error = %v", err)
	}
	defer os.Remove(path)

	_, cmd := m.handleADOEditorFinished(adoEditorFinishedMsg{tempPath: path, task: orig})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when nothing changed", cmd)
	}
}

func TestHandleADOEditorFinishedEmptyTitleDiscardsEdit(t *testing.T) {
	m := NewModel(&config.Config{})
	orig := &task.Task{Title: "Original", Description: "body", ADOItemID: 1}

	tmp, err := os.CreateTemp("", "cmdban-ado-test-*.md")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString("\n\nbody\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	tmp.Close()

	got, cmd := m.handleADOEditorFinished(adoEditorFinishedMsg{tempPath: tmp.Name(), task: orig})
	updated := got.(Model)

	if cmd != nil {
		t.Fatalf("cmd = %v, want nil for an empty title", cmd)
	}
	if updated.message == "" {
		t.Fatalf("message = %q, want an explanation for the discarded edit", updated.message)
	}
}

func TestHandleADOEditorFinishedChangedTitlePushesEdit(t *testing.T) {
	cfg := &config.Config{
		CurrentBoard: "ado",
		Boards: []config.Board{
			{
				Name:        "ado",
				Type:        config.BoardTypeAzureDevOps,
				AzureDevOps: &config.AzureDevOpsConfig{Org: "org", Project: "proj", PAT: "missing"},
			},
		},
	}
	m := NewModel(cfg)
	orig := &task.Task{Title: "Original", Description: "body", ADOItemID: 1}

	path, err := writeADOEditTempFile(&task.Task{Title: "Updated", Description: "body", ADOItemID: 1})
	if err != nil {
		t.Fatalf("writeADOEditTempFile() error = %v", err)
	}
	defer os.Remove(path)

	_, cmd := m.handleADOEditorFinished(adoEditorFinishedMsg{tempPath: path, task: orig})
	if cmd == nil {
		t.Fatalf("cmd = nil, want a push command for a changed title")
	}

	// The configured PAT isn't stored, so the push command should resolve
	// to an error message rather than panic or hang.
	msg := cmd()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("cmd() = %T, want errMsg", msg)
	}
}

func TestSharedGroupKeyPrefersADOParent(t *testing.T) {
	a := &task.Task{Title: "a", ADOParentID: 42, Tags: []string{"backend"}}
	b := &task.Task{Title: "b", ADOParentID: 42, Tags: []string{"frontend"}}

	if got := sharedGroupKey(a, b); got != "parent-42" {
		t.Fatalf("sharedGroupKey() = %q, want %q", got, "parent-42")
	}
}

func TestSharedGroupKeyFallsBackToTag(t *testing.T) {
	a := &task.Task{Title: "a", Tags: []string{"backend"}}
	b := &task.Task{Title: "b", Tags: []string{"backend"}}

	if got := sharedGroupKey(a, b); got != "backend" {
		t.Fatalf("sharedGroupKey() = %q, want %q", got, "backend")
	}
}

func TestSharedGroupKeyNoMatch(t *testing.T) {
	a := &task.Task{Title: "a", ADOParentID: 1, Tags: []string{"backend"}}
	b := &task.Task{Title: "b", ADOParentID: 2, Tags: []string{"frontend"}}

	if got := sharedGroupKey(a, b); got != "" {
		t.Fatalf("sharedGroupKey() = %q, want empty", got)
	}
}

func TestTagGroupColorsGroupsByParent(t *testing.T) {
	tasks := []*task.Task{
		{Title: "a", ADOParentID: 42},
		{Title: "b", ADOParentID: 42},
		{Title: "c", ADOParentID: 7},
	}

	colors := tagGroupColors(tasks)

	if colors[0] == "" || colors[0] != colors[1] {
		t.Fatalf("expected tasks sharing a parent to get the same non-empty bar color, got %v", colors)
	}
	if colors[2] != "" {
		t.Fatalf("expected task with no adjacent parent match to have no bar color, got %q", colors[2])
	}
}

func TestHandleCommandLogsOpensLogsMode(t *testing.T) {
	m := NewModel(&config.Config{})

	got, _ := m.handleCommand("logs")
	updated := got.(Model)

	if updated.mode != ModeLogs {
		t.Fatalf("mode = %v, want ModeLogs", updated.mode)
	}
}

func TestErrMsgIsRecordedInLogs(t *testing.T) {
	m := NewModel(&config.Config{})

	got, _ := m.Update(errMsg{err: errBoom})
	updated := got.(Model)

	entries := updated.logs.Entries()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Level != applog.LevelError {
		t.Fatalf("entries[0].Level = %q, want error", entries[0].Level)
	}
	if entries[0].Message != errBoom.Error() {
		t.Fatalf("entries[0].Message = %q, want %q", entries[0].Message, errBoom.Error())
	}
}

func TestLogsClearEmptiesBuffer(t *testing.T) {
	m := NewModel(&config.Config{})
	m.logs.Error("boom")
	m.mode = ModeLogs

	got, _ := m.handleLogsMode(keyMsgFor("x"))
	updated := got.(Model)

	if len(updated.logs.Entries()) != 0 {
		t.Fatalf("expected logs to be cleared")
	}
}
