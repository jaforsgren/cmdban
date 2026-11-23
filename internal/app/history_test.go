package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cmdban/internal/task"
)

const (
	revisionBefore = 1
	revisionAfter  = 2
	fieldName      = "System.Title"
)

func newTrackedTask(t *testing.T) (*TaskService, *task.Task) {
	t.Helper()
	service := NewTaskService(t.TempDir(), nil, nil, NewHistory())
	created, err := service.Create(testTitle, "", testLane)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return service, created
}

func parsedTitle(t *testing.T, path string) string {
	t.Helper()
	parsed, err := task.ParseMarkdownFile(path)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	return parsed.Title
}

func TestUndoRedoRestoresLocalEdit(t *testing.T) {
	service, created := newTrackedTask(t)
	const renamed = "Renamed"
	if err := service.UpdateTitle(created, renamed); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}

	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := parsedTitle(t, created.FilePath); got != testTitle {
		t.Errorf("after undo Title = %q, want %q", got, testTitle)
	}
	if _, err := service.History().Redo(); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := parsedTitle(t, created.FilePath); got != renamed {
		t.Errorf("after redo Title = %q, want %q", got, renamed)
	}
}

func TestUndoRestoresArchivedTask(t *testing.T) {
	service, created := newTrackedTask(t)
	if err := service.Archive(created); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, err := os.Stat(created.FilePath); err != nil {
		t.Errorf("task not restored: %v", err)
	}
	archived := filepath.Join(filepath.Dir(created.FilePath), "_archive", filepath.Base(created.FilePath))
	if _, err := os.Stat(archived); !os.IsNotExist(err) {
		t.Errorf("archive copy still exists: %v", err)
	}
}

func TestUndoCreateRemovesFile(t *testing.T) {
	service, created := newTrackedTask(t)

	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, err := os.Stat(created.FilePath); !os.IsNotExist(err) {
		t.Errorf("created file still exists: %v", err)
	}
}

func TestUndoRefusesFileChangedSince(t *testing.T) {
	service, created := newTrackedTask(t)
	if err := service.UpdateTitle(created, "Renamed"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}
	external := []byte("# Edited elsewhere\n")
	if err := os.WriteFile(created.FilePath, external, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := service.History().Undo()

	var changed *FileChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("err = %v, want FileChangedError", err)
	}
	content, readErr := os.ReadFile(created.FilePath)
	if readErr != nil || string(content) != string(external) {
		t.Errorf("external edit overwritten: %q, %v", content, readErr)
	}
}

func TestNewChangeClearsRedo(t *testing.T) {
	service, created := newTrackedTask(t)
	if err := service.UpdateTitle(created, "First"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}
	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	reloaded, err := task.ParseMarkdownFile(created.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	if err := service.UpdateTitle(reloaded, "Second"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}

	if _, err := service.History().Redo(); !errors.Is(err, ErrNothingToRedo) {
		t.Errorf("Redo err = %v, want %v", err, ErrNothingToRedo)
	}
}

func TestEmptyHistoryReportsNothingToUndo(t *testing.T) {
	if _, err := NewHistory().Undo(); !errors.Is(err, ErrNothingToUndo) {
		t.Errorf("err = %v, want %v", err, ErrNothingToUndo)
	}
}

func TestRecordFileEditUndoesEditorChange(t *testing.T) {
	service, created := newTrackedTask(t)
	before, err := os.ReadFile(created.FilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(created.FilePath, []byte("# From editor\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := service.RecordFileEdit(created.FilePath, before); err != nil {
		t.Fatalf("RecordFileEdit: %v", err)
	}

	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := parsedTitle(t, created.FilePath); got != testTitle {
		t.Errorf("Title = %q, want %q", got, testTitle)
	}
}

func TestRemoteCommandRestoresWithLatestRevision(t *testing.T) {
	remote := &fakeRemote{restoreRevision: 3}
	command := &remoteCommand{
		remote: remote,
		task:   &task.Task{},
		before: RemoteSnapshot{Revision: revisionBefore, Fields: map[string]any{fieldName: "old"}},
		after:  RemoteSnapshot{Revision: revisionAfter, Fields: map[string]any{fieldName: "new"}},
	}

	if err := command.Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if remote.restoredFrom != revisionAfter {
		t.Errorf("undo tested revision %d, want %d", remote.restoredFrom, revisionAfter)
	}
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if remote.restoredFrom != 3 {
		t.Errorf("redo tested revision %d, want 3", remote.restoredFrom)
	}
}

func TestCompositeUndoRollsBackOnFailure(t *testing.T) {
	first := &countingCommand{}
	second := &countingCommand{undoErr: errRemoteFailed}
	composite := &compositeCommand{commands: []Command{first, second}}

	if err := composite.Undo(); !errors.Is(err, errRemoteFailed) {
		t.Fatalf("err = %v, want %v", err, errRemoteFailed)
	}
	if first.undone != 0 {
		t.Errorf("first undone %d times, want 0", first.undone)
	}

	first, second = &countingCommand{undoErr: errRemoteFailed}, &countingCommand{}
	composite = &compositeCommand{commands: []Command{first, second}}
	if err := composite.Undo(); !errors.Is(err, errRemoteFailed) {
		t.Fatalf("err = %v, want %v", err, errRemoteFailed)
	}
	if second.undone != 1 || second.executed != 1 {
		t.Errorf("second undone %d, executed %d, want 1 and 1", second.undone, second.executed)
	}
}

type countingCommand struct {
	undoErr  error
	undone   int
	executed int
}

func (c *countingCommand) Label() string { return "" }

func (c *countingCommand) Execute() error {
	c.executed++
	return nil
}

func (c *countingCommand) Undo() error {
	if c.undoErr != nil {
		return c.undoErr
	}
	c.undone++
	return nil
}
