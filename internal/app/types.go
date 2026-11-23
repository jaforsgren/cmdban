package app

import "cmdban/internal/task"

// RemoteSnapshot is a work item's undoable fields at one revision, in the
// remote's raw format so restoring them loses nothing.
type RemoteSnapshot struct {
	Revision int
	Fields   map[string]any
}

// RemoteProvider is a backend that owns a board's tasks, such as Azure DevOps.
type RemoteProvider interface {
	Pull() ([]*task.Task, error)
	Create(title, workItemType string, lane task.Status) (*task.Task, error)
	Move(t *task.Task, lane task.Status) error
	UpdateTitle(t *task.Task, title string) error
	UpdateTitleAndDescription(t *task.Task, title, description string) error
	AddTag(t *task.Task, tag string) error
	Snapshot(t *task.Task) (RemoteSnapshot, error)
	// Restore writes target's fields if the item is still at current's
	// revision, and returns the new revision.
	Restore(t *task.Task, current, target RemoteSnapshot) (int, error)
}

// changeScope names what an operation may touch, for history to snapshot.
type changeScope struct {
	tasks       []*task.Task
	directories []string
}
