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
	ID            string
	Title         string
	Description   string
	Status        Status
	Tags          []string
	Priority      int
	Marked        bool
	Order         int
	CheckboxDone  int
	CheckboxTotal int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FilePath      string
}

type Board struct {
	Tasks      map[Status][]*Task
	Lanes      []Status
	AllColumns []Status
}

func NewBoard() *Board {
	return NewBoardWithColumns(nil, nil)
}

func NewBoardWithHiddenColumns(hiddenColumns []string) *Board {
	return NewBoardWithColumns(nil, hiddenColumns)
}

func NewBoardWithColumns(visibleColumns []string, hiddenColumns []string) *Board {
	allColumns := DefaultLanes

	if len(visibleColumns) > 0 {
		allColumns = make([]Status, len(visibleColumns))
		for i, col := range visibleColumns {
			allColumns[i] = Status(col)
		}
	}

	var visibleLanes []Status
	for _, lane := range allColumns {
		if !isColumnHidden(string(lane), hiddenColumns) {
			visibleLanes = append(visibleLanes, lane)
		}
	}

	b := &Board{
		Tasks:      make(map[Status][]*Task),
		Lanes:      visibleLanes,
		AllColumns: allColumns,
	}
	for _, lane := range allColumns {
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
	if _, exists := b.Tasks[t.Status]; !exists {
		b.Tasks[t.Status] = []*Task{}
	}
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
	if _, exists := b.Tasks[newStatus]; !exists {
		b.Tasks[newStatus] = []*Task{}
	}
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
