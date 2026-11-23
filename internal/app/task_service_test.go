package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cmdban/internal/task"
)

const (
	testTitle = "Write tests"
	testLane  = task.Status("today")
)

var errRemoteFailed = errors.New("remote failed")

type fakeRemote struct {
	pulled     []*task.Task
	err        error
	movedTo    task.Status
	titleCalls int

	restoreRevision int
	restoredFrom    int
}

func (f *fakeRemote) Snapshot(t *task.Task) (RemoteSnapshot, error) {
	return RemoteSnapshot{Fields: map[string]any{"title": t.Title}}, nil
}

func (f *fakeRemote) Restore(_ *task.Task, current, _ RemoteSnapshot) (int, error) {
	f.restoredFrom = current.Revision
	return f.restoreRevision, f.err
}

func (f *fakeRemote) Pull() ([]*task.Task, error) { return f.pulled, f.err }

func (f *fakeRemote) Create(title, _ string, lane task.Status) (*task.Task, error) {
	return &task.Task{Title: title, Status: lane}, f.err
}

func (f *fakeRemote) Move(_ *task.Task, lane task.Status) error {
	f.movedTo = lane
	return f.err
}

func (f *fakeRemote) UpdateTitle(*task.Task, string) error {
	f.titleCalls++
	return f.err
}

func (f *fakeRemote) UpdateTitleAndDescription(*task.Task, string, string) error { return f.err }

func (f *fakeRemote) AddTag(t *task.Task, tag string) error {
	if f.err == nil {
		t.Tags = append(t.Tags, tag)
	}
	return f.err
}

func newLocalTask(t *testing.T) (*TaskService, *task.Task) {
	t.Helper()
	service := NewTaskService(t.TempDir(), nil, nil, NewHistory())
	created, err := service.Create(testTitle, "", testLane)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return service, created
}

func TestLocalCreatePersistsLane(t *testing.T) {
	_, created := newLocalTask(t)

	parsed, err := task.ParseMarkdownFile(created.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	if parsed.Status != testLane {
		t.Errorf("Status = %q, want %q", parsed.Status, testLane)
	}
}

func TestLocalUpdateTitlePersists(t *testing.T) {
	service, created := newLocalTask(t)
	const renamed = "Renamed"

	if err := service.UpdateTitle(created, renamed); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}
	parsed, err := task.ParseMarkdownFile(created.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	if parsed.Title != renamed {
		t.Errorf("Title = %q, want %q", parsed.Title, renamed)
	}
}

func TestSaveOrderNumbersFromOne(t *testing.T) {
	service, first := newLocalTask(t)
	second, err := service.Create("Second", "", testLane)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := service.SaveOrder([]*task.Task{second, first}); err != nil {
		t.Fatalf("SaveOrder: %v", err)
	}
	if second.Order != 1 || first.Order != 2 {
		t.Errorf("orders = %d, %d, want 1, 2", second.Order, first.Order)
	}
}

func TestArchiveMovesFileOutOfBoard(t *testing.T) {
	service, created := newLocalTask(t)

	if err := service.Archive(created); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	loaded, err := service.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("loaded %d tasks after archive, want 0", len(loaded))
	}
}

func TestTransferRemovesLocalSource(t *testing.T) {
	service, created := newLocalTask(t)
	target := t.TempDir()

	if err := service.TransferToDirectory(created, target, testLane, ""); err != nil {
		t.Fatalf("TransferToDirectory: %v", err)
	}
	if _, err := os.Stat(created.FilePath); !os.IsNotExist(err) {
		t.Errorf("source still exists: %v", err)
	}
	copies, err := filepath.Glob(filepath.Join(target, "*.md"))
	if err != nil || len(copies) != 1 {
		t.Errorf("copies = %v, err = %v, want one file", copies, err)
	}
}

func TestTransferKeepsRemoteTaskOnRemote(t *testing.T) {
	remote := &fakeRemote{}
	service := NewTaskService(t.TempDir(), nil, remote, NewHistory())
	source := &task.Task{Title: testTitle, Status: testLane}

	if err := service.TransferToDirectory(source, t.TempDir(), testLane, ""); err != nil {
		t.Fatalf("TransferToDirectory: %v", err)
	}
}

func TestRemoteMoveDelegates(t *testing.T) {
	remote := &fakeRemote{}
	service := NewTaskService(t.TempDir(), nil, remote, NewHistory())

	if err := service.PersistMove(&task.Task{}, testLane); err != nil {
		t.Fatalf("PersistMove: %v", err)
	}
	if remote.movedTo != testLane {
		t.Errorf("movedTo = %q, want %q", remote.movedTo, testLane)
	}
}

func TestRemoteTitleFailureLeavesTaskUnchanged(t *testing.T) {
	remote := &fakeRemote{err: errRemoteFailed}
	service := NewTaskService(t.TempDir(), nil, remote, NewHistory())
	source := &task.Task{Title: testTitle}

	err := service.UpdateTitle(source, "Changed")
	if !errors.Is(err, errRemoteFailed) {
		t.Fatalf("err = %v, want %v", err, errRemoteFailed)
	}
	if source.Title != testTitle {
		t.Errorf("Title = %q, want %q", source.Title, testTitle)
	}
}

func TestRemoteDescriptionFailureLeavesTaskUnchanged(t *testing.T) {
	remote := &fakeRemote{err: errRemoteFailed}
	service := NewTaskService(t.TempDir(), nil, remote, NewHistory())
	source := &task.Task{Title: testTitle, Description: "Original"}

	if err := service.UpdateTitleAndDescription(source, "Changed", "Changed"); !errors.Is(err, errRemoteFailed) {
		t.Fatalf("err = %v, want %v", err, errRemoteFailed)
	}
	if source.Title != testTitle || source.Description != "Original" {
		t.Errorf("task changed after failed update: %+v", source)
	}
}
