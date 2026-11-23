package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/app"
)

const (
	undoKey = "u"
	redoKey = "ctrl+r"

	undoVerb = "Undo"
	redoVerb = "Redo"

	nothingToUndoMessage = "Nothing to undo"
	nothingToRedoMessage = "Nothing to redo"
)

type historyAppliedMsg struct {
	verb  string
	label string
	err   error
}

func (m Model) handlesHistoryKeys() bool {
	return m.mode == ModeNormal || m.mode == ModeFocus || m.mode == ModeView
}

func (m Model) undoLastChange() (tea.Model, tea.Cmd) {
	return m.applyHistory(undoVerb, m.taskService().History().Undo)
}

func (m Model) redoLastChange() (tea.Model, tea.Cmd) {
	return m.applyHistory(redoVerb, m.taskService().History().Redo)
}

func (m Model) applyHistory(verb string, apply func() (string, error)) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		label, err := apply()
		return historyAppliedMsg{verb: verb, label: label, err: err}
	}
}

func (m Model) handleHistoryApplied(msg historyAppliedMsg) (tea.Model, tea.Cmd) {
	switch {
	case errors.Is(msg.err, app.ErrNothingToUndo):
		m.message = nothingToUndoMessage
		return m, nil
	case errors.Is(msg.err, app.ErrNothingToRedo):
		m.message = nothingToRedoMessage
		return m, nil
	case msg.err != nil:
		m.message = fmt.Sprintf("%s failed for %s: %s", msg.verb, msg.label, msg.err)
		m.logs.Error(m.message)
	default:
		m.message = fmt.Sprintf("%s: %s", msg.verb, msg.label)
	}
	return m, m.loadTasks
}

func (m Model) handleEditorFinished(msg editorFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.contentBefore != nil {
		if err := m.taskService().RecordFileEdit(msg.filePath, msg.contentBefore); err != nil {
			m.logs.Error("Error recording edit for undo: " + err.Error())
		}
	}
	return m, m.loadTasks
}
