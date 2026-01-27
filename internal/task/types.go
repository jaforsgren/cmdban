package task

import (
	"strings"
	"time"
)

type Status string

const (
	StatusToday    Status = "today"
	StatusTomorrow Status = "tomorrow"
	StatusBacklog  Status = "backlog"
	StatusDone     Status = "done"
)

var DefaultLanes = []Status{StatusToday, StatusTomorrow, StatusBacklog, StatusDone}

type Task struct {
	ID          string
	Title       string
	Description string
	Status      Status
	Tags        []string
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	FilePath    string
}

type Board struct {
	Tasks map[Status][]*Task
	Lanes []Status
}

func NewBoard() *Board {
	return NewBoardWithHiddenColumns(nil)
}

func NewBoardWithHiddenColumns(hiddenColumns []string) *Board {
	var visibleLanes []Status
	for _, lane := range DefaultLanes {
		if !isColumnHidden(string(lane), hiddenColumns) {
			visibleLanes = append(visibleLanes, lane)
		}
	}

	b := &Board{
		Tasks: make(map[Status][]*Task),
		Lanes: visibleLanes,
	}
	for _, lane := range DefaultLanes {
		b.Tasks[lane] = []*Task{}
	}
	return b
}

func isColumnHidden(column string, hiddenColumns []string) bool {
	for _, hidden := range hiddenColumns {
		if strings.EqualFold(hidden, column) {
			return true
		}
	}
	return false
}

func (b *Board) AddTask(t *Task) {
	b.Tasks[t.Status] = append(b.Tasks[t.Status], t)
}

func (b *Board) MoveTask(t *Task, newStatus Status) {
	oldStatus := t.Status
	for i, task := range b.Tasks[oldStatus] {
		if task.ID == t.ID {
			b.Tasks[oldStatus] = append(b.Tasks[oldStatus][:i], b.Tasks[oldStatus][i+1:]...)
			break
		}
	}
	t.Status = newStatus
	t.UpdatedAt = time.Now()
	b.Tasks[newStatus] = append(b.Tasks[newStatus], t)
}

func (b *Board) RemoveTask(t *Task) {
	for i, task := range b.Tasks[t.Status] {
		if task.ID == t.ID {
			b.Tasks[t.Status] = append(b.Tasks[t.Status][:i], b.Tasks[t.Status][i+1:]...)
			return
		}
	}
}
