package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"cmdban/internal/task"
)

// BulkMove moves tasks to lane as one undo step, skipping those already there.
func (s *TaskService) BulkMove(tasks []*task.Task, lane task.Status) error {
	moving := slices.DeleteFunc(slices.Clone(tasks), func(t *task.Task) bool { return t.Status == lane })
	label := fmt.Sprintf("move %d tasks to %s", len(moving), lane)
	return s.track(label, s.scopeFor(moving...), func() error {
		for _, t := range moving {
			if s.remote != nil {
				if err := s.remote.Move(t, lane); err != nil {
					return err
				}
				continue
			}
			t.Status = lane
			t.UpdatedAt = time.Now()
			if err := task.WriteMarkdownFile(t); err != nil {
				return err
			}
		}
		return nil
	})
}

// BulkAddTag tags tasks as one undo step, skipping those that have the tag.
func (s *TaskService) BulkAddTag(tasks []*task.Task, tag string) error {
	tagging := slices.DeleteFunc(slices.Clone(tasks), func(t *task.Task) bool { return s.hasTag(t, tag) })
	label := fmt.Sprintf("tag %d tasks with %s", len(tagging), tag)
	return s.track(label, s.scopeFor(tagging...), func() error {
		for _, t := range tagging {
			if s.remote != nil {
				if err := s.remote.AddTag(t, tag); err != nil {
					return err
				}
				continue
			}
			t.Tags = append(t.Tags, tag)
			if err := task.WriteMarkdownFile(t); err != nil {
				return err
			}
		}
		return nil
	})
}

// BulkSetPriority sets the priority markdown tasks store as one undo step.
func (s *TaskService) BulkSetPriority(tasks []*task.Task, priority int) error {
	label := fmt.Sprintf("set priority %d on %d tasks", priority, len(tasks))
	return s.track(label, changeScope{directories: []string{s.directory}}, func() error {
		for _, t := range tasks {
			t.Priority = priority
			if err := task.WriteMarkdownFile(t); err != nil {
				return err
			}
		}
		return nil
	})
}

// hasTag checks ADOTags on remote boards, because Tags there also carries the
// work item type and assignee for display.
func (s *TaskService) hasTag(t *task.Task, tag string) bool {
	tags := t.Tags
	if s.remote != nil {
		tags = t.ADOTags
	}
	return slices.ContainsFunc(tags, func(existing string) bool { return strings.EqualFold(existing, tag) })
}
