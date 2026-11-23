package app

import (
	"reflect"

	"cmdban/internal/task"
)

// remoteCommand restores a work item's fields. Each write is conditional on
// the revision history last saw, so a newer change by anyone is never
// overwritten.
type remoteCommand struct {
	label  string
	remote RemoteProvider
	task   *task.Task
	before RemoteSnapshot
	after  RemoteSnapshot
}

func (c *remoteCommand) Label() string { return c.label }

func (c *remoteCommand) Execute() error {
	revision, err := c.remote.Restore(c.task, c.before, c.after)
	if err != nil {
		return err
	}
	c.after.Revision = revision
	return nil
}

func (c *remoteCommand) Undo() error {
	revision, err := c.remote.Restore(c.task, c.after, c.before)
	if err != nil {
		return err
	}
	c.before.Revision = revision
	return nil
}

func snapshotsDiffer(before, after RemoteSnapshot) bool {
	return !reflect.DeepEqual(before.Fields, after.Fields)
}
