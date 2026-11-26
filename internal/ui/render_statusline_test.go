package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func plainFooter(m Model) string {
	return ansi.Strip(m.renderFooter())
}

func TestStatusLineShowsModeTasksBoardAndSync(t *testing.T) {
	m, _ := newVisualModel(t, "one", "two", "three")
	m.lastSyncTime = time.Now().Add(-12 * time.Second)

	footer := plainFooter(m)

	for _, want := range []string{"NORMAL", "3 tasks", visualTestBoard, "synced 12s ago"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer = %q, want it to contain %q", footer, want)
		}
	}
	if strings.Contains(footer, "marked") {
		t.Errorf("footer = %q, want no marked count while nothing is marked", footer)
	}
}

func TestStatusLineCountsMarkedTasks(t *testing.T) {
	m, tasks := newVisualModel(t, "one", "two", "three")
	tasks[0].Marked = true
	tasks[2].Marked = true

	if footer := plainFooter(m); !strings.Contains(footer, "2 marked") {
		t.Fatalf("footer = %q, want 2 marked", footer)
	}
}

func TestStatusLineShowsSearchFilterAndFilteredCount(t *testing.T) {
	m, _ := newVisualModel(t, "auth login", "auth logout", "docs")
	m.searchFilter = "auth"

	footer := plainFooter(m)
	for _, want := range []string{"2 of 3 tasks", "/auth"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer = %q, want it to contain %q", footer, want)
		}
	}
}

func TestStatusLineAppendsMessage(t *testing.T) {
	m, _ := newVisualModel(t, "one")
	m.message = "Saved"

	if footer := plainFooter(m); !strings.HasSuffix(strings.TrimSpace(footer), statusLineSeparator+"Saved") {
		t.Fatalf("footer = %q, want it to end with the message", footer)
	}
}

func TestStatusLineMarksLiveReload(t *testing.T) {
	m, _ := newVisualModel(t, "one")
	m.liveReload = true

	if footer := plainFooter(m); !strings.Contains(footer, "· live") {
		t.Fatalf("footer = %q, want the live reload marker", footer)
	}
}

func TestStatusLineShowsVisualSelection(t *testing.T) {
	m, _ := newVisualModel(t, "one", "two", "three")
	m = sendVisualKey(t, m, "V")
	m = sendVisualKey(t, m, "j")

	footer := plainFooter(m)
	for _, want := range []string{"VISUAL", "2 selected"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer = %q, want it to contain %q", footer, want)
		}
	}
}

func TestStatusLineStaysOneRowOnNarrowTerminal(t *testing.T) {
	m, _ := newVisualModel(t, "one")
	m.width = 30
	m.message = strings.Repeat("long message ", 10)

	footer := m.renderFooter()
	if height := lipgloss.Height(footer); height != 1 {
		t.Fatalf("footer height = %d, want 1", height)
	}
	if width := lipgloss.Width(footer); width > m.width {
		t.Fatalf("footer width = %d, want at most %d", width, m.width)
	}
}

func TestCommandLineIsOneRowWithColonPrefix(t *testing.T) {
	m, _ := newVisualModel(t, "one")
	m = sendVisualKey(t, m, ":")
	m.textInput.SetValue("move done")

	footer := m.renderFooter()
	if height := lipgloss.Height(footer); height != 1 {
		t.Fatalf("command line height = %d, want 1", height)
	}
	if plain := ansi.Strip(footer); !strings.Contains(plain, ":move done") {
		t.Fatalf("command line = %q, want :move done", plain)
	}
}

func TestSearchLineIsOneRowWithSlashPrefix(t *testing.T) {
	m, _ := newVisualModel(t, "authentication")
	m = sendVisualKey(t, m, "/")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("auth")})
	m = updated.(Model)

	footer := m.renderFooter()
	if height := lipgloss.Height(footer); height != 1 {
		t.Fatalf("search line height = %d, want 1", height)
	}
	if plain := ansi.Strip(footer); !strings.Contains(plain, "/auth") {
		t.Fatalf("search line = %q, want /auth", plain)
	}
}

func TestStatusLineInWikiModeShowsWikisInsteadOfTasks(t *testing.T) {
	m, _ := newVisualModel(t, "one")
	m.mode = ModeWiki

	footer := plainFooter(m)
	for _, want := range []string{"WIKI", "0 wikis", "synced never"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer = %q, want it to contain %q", footer, want)
		}
	}
	if strings.Contains(footer, "tasks") {
		t.Errorf("footer = %q, want no task count in wiki mode", footer)
	}
}

func TestStatusClockReschedulesItself(t *testing.T) {
	m, _ := newVisualModel(t, "one")

	_, cmd := m.Update(statusClockMsg{})
	if cmd == nil {
		t.Fatal("statusClockMsg returned no command, want the next tick scheduled")
	}
}
