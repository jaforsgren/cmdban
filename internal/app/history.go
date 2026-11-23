package app

import (
	"errors"
	"sync"
)

const historyLimit = 100

var (
	ErrNothingToUndo = errors.New("nothing to undo")
	ErrNothingToRedo = errors.New("nothing to redo")
)

// Command is a recorded change that can be reversed and applied again.
type Command interface {
	Label() string
	Execute() error
	Undo() error
}

// History holds one board's undo and redo stacks. Its lock also serializes
// tracked operations, so concurrent writes can't land in each other's
// before/after snapshots.
type History struct {
	mu        sync.Mutex
	undoStack []Command
	redoStack []Command
}

func NewHistory() *History {
	return &History{}
}

func (h *History) recordLocked(command Command) {
	h.undoStack = append(h.undoStack, command)
	if len(h.undoStack) > historyLimit {
		h.undoStack = h.undoStack[len(h.undoStack)-historyLimit:]
	}
	h.redoStack = nil
}

// Undo reverses the latest command. A command that fails stays on the stack.
func (h *History) Undo() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.undoStack) == 0 {
		return "", ErrNothingToUndo
	}
	command := h.undoStack[len(h.undoStack)-1]
	if err := command.Undo(); err != nil {
		return command.Label(), err
	}
	h.undoStack = h.undoStack[:len(h.undoStack)-1]
	h.redoStack = append(h.redoStack, command)
	return command.Label(), nil
}

// Redo reapplies the latest undone command. A command that fails stays on the
// stack.
func (h *History) Redo() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.redoStack) == 0 {
		return "", ErrNothingToRedo
	}
	command := h.redoStack[len(h.redoStack)-1]
	if err := command.Execute(); err != nil {
		return command.Label(), err
	}
	h.redoStack = h.redoStack[:len(h.redoStack)-1]
	h.undoStack = append(h.undoStack, command)
	return command.Label(), nil
}

// HistoryRegistry keeps a separate history per board for the whole session.
type HistoryRegistry struct {
	mu        sync.Mutex
	histories map[string]*History
}

func NewHistoryRegistry() *HistoryRegistry {
	return &HistoryRegistry{histories: make(map[string]*History)}
}

func (r *HistoryRegistry) For(board string) *History {
	r.mu.Lock()
	defer r.mu.Unlock()
	history, ok := r.histories[board]
	if !ok {
		history = NewHistory()
		r.histories[board] = history
	}
	return history
}

// compositeCommand applies several commands as one undo step. If one fails,
// those already applied are rolled back so the step stays all-or-nothing.
type compositeCommand struct {
	label    string
	commands []Command
}

func (c *compositeCommand) Label() string { return c.label }

func (c *compositeCommand) Execute() error {
	for i, command := range c.commands {
		if err := command.Execute(); err != nil {
			errs := []error{err}
			for j := i - 1; j >= 0; j-- {
				errs = append(errs, c.commands[j].Undo())
			}
			return errors.Join(errs...)
		}
	}
	return nil
}

func (c *compositeCommand) Undo() error {
	for i := len(c.commands) - 1; i >= 0; i-- {
		if err := c.commands[i].Undo(); err != nil {
			errs := []error{err}
			for j := i + 1; j < len(c.commands); j++ {
				errs = append(errs, c.commands[j].Execute())
			}
			return errors.Join(errs...)
		}
	}
	return nil
}
