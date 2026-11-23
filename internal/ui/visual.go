package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/task"
)

const (
	visualModeKey     = "V"
	visualMoveKey     = "m"
	visualPriorityKey = "p"
	visualTagKey      = "t"
	visualArchiveKey  = "a"
	visualDeleteKey   = "d"
	visualDownKey     = "j"
	visualUpKey       = "k"
	visualExitKey     = "esc"

	visualSelectionMarker = "▸ "

	inputActionBulkMove     = "bulk-move"
	inputActionBulkPriority = "bulk-priority"
	inputActionBulkTag      = "bulk-tag"
	inputActionBulkArchive  = "bulk-archive"
)

// visualState is the range being selected. The anchor is an index into the
// active lane's filtered tasks, like activeTask.
type visualState struct {
	active bool
	anchor int
}

// pendingBulk holds a bulk action awaiting confirmation. It stores IDs because
// a sync during the prompt replaces the board's task pointers.
type pendingBulk struct {
	action  string
	taskIDs []string
	value   string
}

type bulkAppliedMsg struct {
	summary string
	err     error
}

func isBulkAction(action string) bool {
	switch action {
	case inputActionBulkMove, inputActionBulkPriority, inputActionBulkTag, inputActionBulkArchive:
		return true
	}
	return false
}

func (m Model) enterVisualMode() (tea.Model, tea.Cmd) {
	if m.selectedTask() == nil {
		return m, nil
	}
	m.visual = visualState{active: true, anchor: m.activeTask}
	m.mode = ModeVisual
	m.gPressed = false
	m.message = ""
	return m, nil
}

func (m *Model) exitVisualMode() {
	m.visual = visualState{}
	m.pendingBulk = pendingBulk{}
	m.mode = ModeNormal
}

func (m Model) handleVisualMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case visualExitKey, visualModeKey:
		m.exitVisualMode()
	case visualDownKey, "down":
		m.activeTask++
		m.clampSelection()
	case visualUpKey, "up":
		m.activeTask--
		m.clampSelection()
	case visualMoveKey:
		return m.startBulkInput(inputActionBulkMove, "Move to lane...")
	case visualTagKey:
		return m.startBulkInput(inputActionBulkTag, "Tag...")
	case visualPriorityKey:
		if m.config.IsADOBoard() {
			m.message = "Priority is not supported for Azure DevOps boards"
			return m, nil
		}
		return m.startBulkInput(inputActionBulkPriority, "Priority (0 clears)...")
	case visualArchiveKey, visualDeleteKey:
		if m.config.IsADOBoard() {
			m.message = "Use Azure DevOps to delete work items"
			return m, nil
		}
		m.pendingBulk = pendingBulk{action: inputActionBulkArchive, taskIDs: taskIDs(m.bulkTargets())}
		m.inputAction = inputActionBulkArchive
		m.mode = ModeConfirm
	}
	return m, nil
}

func (m Model) startBulkInput(action, placeholder string) (tea.Model, tea.Cmd) {
	m.pendingBulk = pendingBulk{action: action, taskIDs: taskIDs(m.bulkTargets())}
	m.mode = ModeInput
	m.inputAction = action
	m.inputError = ""
	m.textInput.Reset()
	m.textInput.Placeholder = placeholder
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Focus()
	return m, textinput.Blink
}

// visualRange is the selected span of the active lane's filtered tasks,
// clamped because a sync can shorten the lane during selection.
func (m Model) visualRange(tasks []*task.Task) []*task.Task {
	if !m.visual.active || len(tasks) == 0 {
		return nil
	}
	last := len(tasks) - 1
	anchor, cursor := min(m.visual.anchor, last), min(m.activeTask, last)
	return tasks[min(anchor, cursor) : max(anchor, cursor)+1]
}

// bulkTargets is the visual range plus every visible marked card.
func (m Model) bulkTargets() []*task.Task {
	activeLane := m.board.Lanes[m.activeLane]
	inRange := m.visualRange(m.filterTasks(m.board.Tasks[activeLane]))

	var targets []*task.Task
	for _, lane := range m.board.Lanes {
		for _, t := range m.filterTasks(m.board.Tasks[lane]) {
			if t.Marked || slices.Contains(inRange, t) {
				targets = append(targets, t)
			}
		}
	}
	return targets
}

func (m Model) isVisuallySelected(t *task.Task) bool {
	return m.visual.active && slices.Contains(m.bulkTargets(), t)
}

func taskIDs(tasks []*task.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

func (m Model) pendingBulkTasks() []*task.Task {
	var tasks []*task.Task
	for _, lane := range m.board.AllColumns {
		for _, t := range m.board.Tasks[lane] {
			if slices.Contains(m.pendingBulk.taskIDs, t.ID) {
				tasks = append(tasks, t)
			}
		}
	}
	return tasks
}

// submitBulkInput validates a prompt's value. ADO writes then ask for
// confirmation, matching single-card moves, since they change items for
// everyone.
func (m Model) submitBulkInput(value string) (tea.Model, tea.Cmd) {
	switch m.inputAction {
	case inputActionBulkMove:
		lane, ok := m.laneNamed(value)
		if !ok {
			m.inputError = fmt.Sprintf("No lane named %q", value)
			return m, nil
		}
		value = string(lane)
	case inputActionBulkPriority:
		if priority, err := strconv.Atoi(value); err != nil || priority < 0 {
			m.inputError = "Priority must be a whole number, 0 or higher"
			return m, nil
		}
	}
	m.pendingBulk.value = value
	m.textInput.Reset()
	if m.config.IsADOBoard() {
		m.mode = ModeConfirm
		return m, nil
	}
	return m.applyBulk()
}

func (m Model) laneNamed(name string) (task.Status, bool) {
	index := slices.IndexFunc(m.board.AllColumns, func(lane task.Status) bool {
		return strings.EqualFold(string(lane), name)
	})
	if index < 0 {
		return "", false
	}
	return m.board.AllColumns[index], true
}

func (m Model) applyBulk() (tea.Model, tea.Cmd) {
	pending := m.pendingBulk
	tasks := m.pendingBulkTasks()
	m.exitVisualMode()
	if len(tasks) == 0 {
		m.message = "The selected tasks are no longer on the board"
		return m, nil
	}

	service := m.taskService()
	summary := bulkSummary(pending, len(tasks))
	m.message = "Applying: " + summary
	return m, func() tea.Msg {
		var err error
		switch pending.action {
		case inputActionBulkMove:
			err = service.BulkMove(tasks, task.Status(pending.value))
		case inputActionBulkTag:
			err = service.BulkAddTag(tasks, pending.value)
		case inputActionBulkPriority:
			var priority int
			if priority, err = strconv.Atoi(pending.value); err == nil {
				err = service.BulkSetPriority(tasks, priority)
			}
		case inputActionBulkArchive:
			err = service.Archive(tasks...)
		}
		return bulkAppliedMsg{summary: summary, err: err}
	}
}

func bulkSummary(pending pendingBulk, count int) string {
	switch pending.action {
	case inputActionBulkMove:
		return fmt.Sprintf("move %d tasks to %s", count, pending.value)
	case inputActionBulkTag:
		return fmt.Sprintf("tag %d tasks with %s", count, pending.value)
	case inputActionBulkPriority:
		return fmt.Sprintf("set priority %s on %d tasks", pending.value, count)
	}
	return fmt.Sprintf("archive %d tasks", count)
}

// handleBulkApplied always reloads, since a failed bulk action may have
// changed some of its tasks. Those changes stay undoable.
func (m Model) handleBulkApplied(msg bulkAppliedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.message = fmt.Sprintf("Failed to %s: %s", msg.summary, msg.err)
		m.logs.Error(m.message)
		return m, m.loadTasks
	}
	m.message = fmt.Sprintf("Done: %s (%s undoes)", msg.summary, undoKey)
	return m, m.loadTasks
}

func (m Model) bulkConfirmPrompt() string {
	count := len(m.pendingBulk.taskIDs)
	if m.pendingBulk.action == inputActionBulkArchive {
		return fmt.Sprintf("Archive %d tasks? (y/n)", count)
	}
	return fmt.Sprintf("Apply to %d work items in Azure DevOps: %s? (y/n)", count, bulkSummary(m.pendingBulk, count))
}

func (m Model) visualStatusLine() string {
	return fmt.Sprintf("VISUAL  %d selected", len(m.bulkTargets()))
}
