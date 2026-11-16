package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// wikiWatchFetchLimit caps how many cached pages a watch refresh refetches
// at once.
const wikiWatchFetchLimit = 4

// wikiWatchTickMsg carries the generation it was scheduled under, so ticks
// from a watch that was toggled off (or restarted) are dropped.
type wikiWatchTickMsg struct {
	generation int
}

// toggleWikiWatch turns on refreshing the wikis every live reload interval,
// the wiki counterpart of the board's live reload.
func (m Model) toggleWikiWatch() (tea.Model, tea.Cmd) {
	m.wiki.watch = !m.wiki.watch
	m.wiki.watchGeneration++
	if !m.wiki.watch {
		m.message = "Wiki watch off"
		return m, nil
	}
	m.message = fmt.Sprintf("Wiki watch on — refreshing every %s", m.config.LiveReloadInterval())
	return m, m.scheduleWikiWatch()
}

func (m Model) scheduleWikiWatch() tea.Cmd {
	generation := m.wiki.watchGeneration
	return tea.Tick(m.config.LiveReloadInterval(), func(time.Time) tea.Msg {
		return wikiWatchTickMsg{generation: generation}
	})
}

func (m Model) handleWikiWatchTick(msg wikiWatchTickMsg) (tea.Model, tea.Cmd) {
	if !m.wiki.watch || msg.generation != m.wiki.watchGeneration {
		return m, nil
	}
	return m, tea.Batch(append(m.refreshWikisInBackground(), m.scheduleWikiWatch())...)
}

// refreshWikisInBackground refetches every page tree and every cached page,
// leaving what is on screen in place until the new copies arrive.
func (m Model) refreshWikisInBackground() []tea.Cmd {
	var cmds []tea.Cmd
	for _, w := range m.config.Wikis {
		cmds = append(cmds, m.fetchWikiTree(w, true))
	}
	limit := make(chan struct{}, wikiWatchFetchLimit)
	for i, w := range m.config.Wikis {
		for key := range m.wiki.pages {
			if key.wiki == wikiID(w) && !m.wiki.loadingPages[key] {
				cmds = append(cmds, m.fetchWikiPage(i, key, limit))
			}
		}
	}
	return cmds
}
