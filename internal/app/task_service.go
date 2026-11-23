package app

import (
	"fmt"
	"os"
	"path/filepath"

	"cmdban/internal/task"
)

// TaskService performs task operations against a board's markdown directory,
// or against its remote when the board has one, recording them in history.
type TaskService struct {
	directory   string
	ignoreFiles []string
	remote      RemoteProvider
	history     *History
}

// NewTaskService returns a service for a local board when remote is nil.
func NewTaskService(directory string, ignoreFiles []string, remote RemoteProvider, history *History) *TaskService {
	return &TaskService{directory: directory, ignoreFiles: ignoreFiles, remote: remote, history: history}
}

func (s *TaskService) History() *History {
	return s.history
}

func (s *TaskService) Load() ([]*task.Task, error) {
	if s.remote != nil {
		return s.remote.Pull()
	}
	return task.LoadTasksFromDirectory(s.directory, s.ignoreFiles)
}

// Create adds a task to lane; workItemType applies to remote boards only.
// Remote creation isn't tracked because work items can't be cleanly deleted.
func (s *TaskService) Create(title, workItemType string, lane task.Status) (*task.Task, error) {
	if s.remote != nil {
		return s.remote.Create(title, workItemType, lane)
	}
	var created *task.Task
	err := s.track(fmt.Sprintf("create %q", title), s.scopeFor(), func() error {
		t, err := task.CreateNewTask(s.directory, title)
		if err != nil {
			return err
		}
		t.Status = lane
		if err := task.WriteMarkdownFile(t); err != nil {
			return err
		}
		created = t
		return nil
	})
	return created, err
}

func (s *TaskService) UpdateTitle(t *task.Task, title string) error {
	return s.track(fmt.Sprintf("rename %q", t.Title), s.scopeFor(t), func() error {
		if s.remote != nil {
			if err := s.remote.UpdateTitle(t, title); err != nil {
				return err
			}
			t.Title = title
			return nil
		}
		t.Title = title
		return task.WriteMarkdownFile(t)
	})
}

func (s *TaskService) UpdateTitleAndDescription(t *task.Task, title, description string) error {
	return s.track(fmt.Sprintf("edit %q", t.Title), s.scopeFor(t), func() error {
		if s.remote != nil {
			if err := s.remote.UpdateTitleAndDescription(t, title, description); err != nil {
				return err
			}
		}
		t.Title = title
		t.Description = description
		if s.remote != nil {
			return nil
		}
		return task.WriteMarkdownFile(t)
	})
}

// ApplyTag shows a tag on a local task right away. Remote tasks change only
// once PersistTag succeeds, because the remote can reject the write.
func (s *TaskService) ApplyTag(t *task.Task, tag string) {
	if s.remote == nil {
		t.Tags = append(t.Tags, tag)
	}
}

// PersistTag saves a tag, after ApplyTag on local boards.
func (s *TaskService) PersistTag(t *task.Task, tag string) error {
	return s.track(fmt.Sprintf("tag %q", t.Title), s.scopeFor(t), func() error {
		if s.remote != nil {
			return s.remote.AddTag(t, tag)
		}
		return task.WriteMarkdownFile(t)
	})
}

// PersistMove saves a move the board has already applied to t.
func (s *TaskService) PersistMove(t *task.Task, lane task.Status) error {
	return s.track(fmt.Sprintf("move %q to %s", t.Title, lane), s.scopeFor(t), func() error {
		if s.remote != nil {
			return s.remote.Move(t, lane)
		}
		return task.WriteMarkdownFile(t)
	})
}

// SaveMetadata persists fields only the markdown file stores, such as marks.
func (s *TaskService) SaveMetadata(t *task.Task) error {
	return s.track(fmt.Sprintf("update %q", t.Title), changeScope{directories: []string{s.directory}}, func() error {
		return task.WriteMarkdownFile(t)
	})
}

// SaveOrder numbers tasks by their position in the lane.
func (s *TaskService) SaveOrder(tasks []*task.Task) error {
	return s.track("reorder lane", changeScope{directories: []string{s.directory}}, func() error {
		for i, t := range tasks {
			t.Order = i + 1
			if err := task.WriteMarkdownFile(t); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *TaskService) Archive(tasks ...*task.Task) error {
	return s.track(archiveLabel(tasks), changeScope{directories: []string{s.directory}}, func() error {
		for _, t := range tasks {
			if err := task.ArchiveTask(t); err != nil {
				return err
			}
		}
		return nil
	})
}

func archiveLabel(tasks []*task.Task) string {
	if len(tasks) == 1 {
		return fmt.Sprintf("archive %q", tasks[0].Title)
	}
	return fmt.Sprintf("archive %d tasks", len(tasks))
}

// TransferToDirectory copies t into another board's directory. Remote tasks
// keep existing on their remote, so only local source files are removed.
func (s *TaskService) TransferToDirectory(t *task.Task, directory string, status task.Status, sourceURL string) error {
	scope := changeScope{directories: []string{directory}}
	if s.remote == nil {
		scope.directories = append(scope.directories, s.directory)
	}
	return s.track(fmt.Sprintf("move %q to another board", t.Title), scope, func() error {
		if _, err := task.CopyTaskToDirectory(t, directory, status, sourceURL); err != nil {
			return err
		}
		if s.remote != nil {
			return nil
		}
		if err := os.Remove(t.FilePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
}

// RecordFileEdit records an edit made outside cmdban, such as in $EDITOR,
// given the file's content from before the edit.
func (s *TaskService) RecordFileEdit(path string, before []byte) error {
	s.history.mu.Lock()
	defer s.history.mu.Unlock()

	after, err := readOptionalFile(path)
	if err != nil {
		return err
	}
	if sameContent(before, after) {
		return nil
	}
	s.history.recordLocked(&fileCommand{
		label:   "edit " + filepath.Base(path),
		changes: []fileChange{{path: path, before: before, after: after}},
	})
	return nil
}

func (s *TaskService) scopeFor(tasks ...*task.Task) changeScope {
	if s.remote != nil {
		return changeScope{tasks: tasks}
	}
	return changeScope{directories: []string{s.directory}}
}

// track runs op and records what it changed. A failed op that changed
// something is still recorded, so a partial bulk write can be undone.
func (s *TaskService) track(label string, scope changeScope, op func() error) error {
	s.history.mu.Lock()
	defer s.history.mu.Unlock()

	filesBefore, err := captureMarkdownFiles(scope.directories)
	if err != nil {
		return err
	}
	snapshotsBefore := make([]RemoteSnapshot, len(scope.tasks))
	for i, t := range scope.tasks {
		if snapshotsBefore[i], err = s.remote.Snapshot(t); err != nil {
			return err
		}
	}

	opErr := op()

	command, err := s.commandFor(label, scope, filesBefore, snapshotsBefore)
	if command != nil {
		s.history.recordLocked(command)
	}
	if opErr != nil {
		return opErr
	}
	if err != nil {
		return fmt.Errorf("saved, but this action can't be undone: %w", err)
	}
	return nil
}

func (s *TaskService) commandFor(label string, scope changeScope, filesBefore map[string][]byte, snapshotsBefore []RemoteSnapshot) (Command, error) {
	filesAfter, err := captureMarkdownFiles(scope.directories)
	if err != nil {
		return nil, err
	}
	var commands []Command
	if changes := diffFiles(filesBefore, filesAfter); len(changes) > 0 {
		commands = append(commands, &fileCommand{label: label, changes: changes})
	}
	for i, t := range scope.tasks {
		after, err := s.remote.Snapshot(t)
		if err != nil {
			return nil, err
		}
		if snapshotsDiffer(snapshotsBefore[i], after) {
			commands = append(commands, &remoteCommand{label: label, remote: s.remote, task: t, before: snapshotsBefore[i], after: after})
		}
	}
	switch len(commands) {
	case 0:
		return nil, nil
	case 1:
		return commands[0], nil
	}
	return &compositeCommand{label: label, commands: commands}, nil
}
