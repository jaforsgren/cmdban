package app

import (
	"errors"
	"testing"

	"cmdban/internal/task"
)

const bulkTargetLane = task.Status("done")

func newBulkTasks(t *testing.T) (*TaskService, []*task.Task) {
	t.Helper()
	service := NewTaskService(t.TempDir(), nil, nil, NewHistory())
	var tasks []*task.Task
	for _, title := range []string{"First", "Second", "Third"} {
		created, err := service.Create(title, "", testLane)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		tasks = append(tasks, created)
	}
	return service, tasks
}

func parsedTask(t *testing.T, path string) *task.Task {
	t.Helper()
	parsed, err := task.ParseMarkdownFile(path)
	if err != nil {
		t.Fatalf("ParseMarkdownFile: %v", err)
	}
	return parsed
}

func TestBulkMoveUndoesAsOneStep(t *testing.T) {
	service, tasks := newBulkTasks(t)

	if err := service.BulkMove(tasks, bulkTargetLane); err != nil {
		t.Fatalf("BulkMove: %v", err)
	}
	for _, created := range tasks {
		if got := parsedTask(t, created.FilePath).Status; got != bulkTargetLane {
			t.Errorf("%s Status = %q, want %q", created.Title, got, bulkTargetLane)
		}
	}

	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for _, created := range tasks {
		if got := parsedTask(t, created.FilePath).Status; got != testLane {
			t.Errorf("%s Status = %q, want %q", created.Title, got, testLane)
		}
	}
}

func TestBulkAddTagSkipsTasksWithTag(t *testing.T) {
	service, tasks := newBulkTasks(t)
	const tag = "backend"
	if err := service.PersistTag(tasks[0], tag); err != nil {
		t.Fatalf("PersistTag: %v", err)
	}
	tasks[0].Tags = []string{"Backend"}

	if err := service.BulkAddTag(tasks, tag); err != nil {
		t.Fatalf("BulkAddTag: %v", err)
	}

	if len(tasks[0].Tags) != 1 {
		t.Errorf("first Tags = %v, want one tag", tasks[0].Tags)
	}
	if got := parsedTask(t, tasks[1].FilePath).Tags; len(got) != 1 || got[0] != tag {
		t.Errorf("second Tags = %v, want [%s]", got, tag)
	}
}

func TestBulkAddTagIgnoresDisplayTagsOnRemote(t *testing.T) {
	remote := &fakeRemote{}
	service := NewTaskService(t.TempDir(), nil, remote, NewHistory())
	const displayOnlyTag = "Bug"
	workItem := &task.Task{Tags: []string{displayOnlyTag}}

	if err := service.BulkAddTag([]*task.Task{workItem}, displayOnlyTag); err != nil {
		t.Fatalf("BulkAddTag: %v", err)
	}
	if len(workItem.Tags) != 2 {
		t.Errorf("Tags = %v, want the tag sent to the remote", workItem.Tags)
	}
}

func TestBulkSetPriorityUndoes(t *testing.T) {
	service, tasks := newBulkTasks(t)
	const priority = 2

	if err := service.BulkSetPriority(tasks, priority); err != nil {
		t.Fatalf("BulkSetPriority: %v", err)
	}
	if got := parsedTask(t, tasks[2].FilePath).Priority; got != priority {
		t.Errorf("Priority = %d, want %d", got, priority)
	}
	if _, err := service.History().Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := parsedTask(t, tasks[2].FilePath).Priority; got != 0 {
		t.Errorf("Priority after undo = %d, want 0", got)
	}
}

func TestBulkMoveRemoteStopsAtFirstFailure(t *testing.T) {
	remote := &fakeRemote{err: errRemoteFailed}
	service := NewTaskService(t.TempDir(), nil, remote, NewHistory())
	tasks := []*task.Task{{Title: "First", Status: testLane}, {Title: "Second", Status: testLane}}

	if err := service.BulkMove(tasks, bulkTargetLane); !errors.Is(err, errRemoteFailed) {
		t.Fatalf("err = %v, want %v", err, errRemoteFailed)
	}
}
