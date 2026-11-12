package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// liveReloadTickMsg carries the generation it was scheduled under, so ticks
// from a loop that was toggled off (or restarted) are dropped instead of
// running a second loop alongside the new one.
type liveReloadTickMsg struct {
	generation int
}

func (m Model) toggleLiveReload() (tea.Model, tea.Cmd) {
	m.liveReload = !m.liveReload
	m.liveReloadGeneration++
	if !m.liveReload {
		m.message = "Live reload off"
		return m, nil
	}
	interval := m.config.LiveReloadInterval()
	m.message = fmt.Sprintf("Live reload on — syncing every %s", interval)
	return m, m.scheduleLiveReload()
}

func (m Model) scheduleLiveReload() tea.Cmd {
	generation := m.liveReloadGeneration
	return tea.Tick(m.config.LiveReloadInterval(), func(time.Time) tea.Msg {
		return liveReloadTickMsg{generation: generation}
	})
}

func (m Model) handleLiveReloadTick(msg liveReloadTickMsg) (tea.Model, tea.Cmd) {
	if !m.liveReload || msg.generation != m.liveReloadGeneration {
		return m, nil
	}
	return m, tea.Batch(m.loadTasksInBackground, m.scheduleLiveReload())
}

func (m Model) loadTasksInBackground() tea.Msg {
	msg := m.loadTasks()
	if loaded, ok := msg.(tasksLoadedMsg); ok {
		loaded.background = true
		return loaded
	}
	return msg
}
