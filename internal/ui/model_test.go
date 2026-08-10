package ui

import (
	"errors"
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
