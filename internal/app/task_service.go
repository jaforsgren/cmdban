package app

import (
	"os"

	"cmdban/internal/task"
)

// TaskService performs task operations against a board's markdown directory,
// or against its remote when the board has one.
type TaskService struct {
	directory   string
	ignoreFiles []string
	remote      RemoteProvider
}

// NewTaskService returns a service for a local board when remote is nil.
func NewTaskService(directory string, ignoreFiles []string, remote RemoteProvider) *TaskService {
	return &TaskService{directory: directory, ignoreFiles: ignoreFiles, remote: remote}
}

func (s *TaskService) Load() ([]*task.Task, error) {
	if s.remote != nil {
		return s.remote.Pull()
	}
	return task.LoadTasksFromDirectory(s.directory, s.ignoreFiles)
}

// Create adds a task to lane; workItemType applies to remote boards only.
func (s *TaskService) Create(title, workItemType string, lane task.Status) (*task.Task, error) {
	if s.remote != nil {
		return s.remote.Create(title, workItemType, lane)
	}
	t, err := task.CreateNewTask(s.directory, title)
	if err != nil {
		return nil, err
	}
	t.Status = lane
	if err := task.WriteMarkdownFile(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TaskService) UpdateTitle(t *task.Task, title string) error {
	if s.remote != nil {
		if err := s.remote.UpdateTitle(t, title); err != nil {
			return err
		}
		t.Title = title
		return nil
	}
	t.Title = title
	return task.WriteMarkdownFile(t)
}

func (s *TaskService) UpdateTitleAndDescription(t *task.Task, title, description string) error {
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
	if s.remote != nil {
		return s.remote.AddTag(t, tag)
	}
	return task.WriteMarkdownFile(t)
}

// PersistMove saves a move the board has already applied to t.
func (s *TaskService) PersistMove(t *task.Task, lane task.Status) error {
	if s.remote != nil {
		return s.remote.Move(t, lane)
	}
	return task.WriteMarkdownFile(t)
}

// SaveMetadata persists fields only the markdown file stores, such as marks.
func (s *TaskService) SaveMetadata(t *task.Task) error {
	return task.WriteMarkdownFile(t)
}

// SaveOrder numbers tasks by their position in the lane.
func (s *TaskService) SaveOrder(tasks []*task.Task) error {
	for i, t := range tasks {
		t.Order = i + 1
		if err := task.WriteMarkdownFile(t); err != nil {
			return err
		}
	}
	return nil
}

func (s *TaskService) Archive(tasks ...*task.Task) error {
	for _, t := range tasks {
		if err := task.ArchiveTask(t); err != nil {
			return err
		}
	}
	return nil
}

// TransferToDirectory copies t into another board's directory. Remote tasks
// keep existing on their remote, so only local source files are removed.
func (s *TaskService) TransferToDirectory(t *task.Task, directory string, status task.Status, sourceURL string) error {
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
}
