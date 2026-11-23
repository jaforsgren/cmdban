package app

import "cmdban/internal/task"

// RemoteProvider is a backend that owns a board's tasks, such as Azure DevOps.
type RemoteProvider interface {
	Pull() ([]*task.Task, error)
	Create(title, workItemType string, lane task.Status) (*task.Task, error)
	Move(t *task.Task, lane task.Status) error
	UpdateTitle(t *task.Task, title string) error
	UpdateTitleAndDescription(t *task.Task, title, description string) error
	AddTag(t *task.Task, tag string) error
}
