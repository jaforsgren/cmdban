package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/applog"
)

const logsVisibleRows = 15

func (m Model) handleLogsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	count := len(m.logs.Entries())

	switch msg.String() {
	case "j", "down":
		if m.logScroll < count-1 {
			m.logScroll++
		}
	case "k", "up":
		if m.logScroll > 0 {
			m.logScroll--
		}
	case "g":
		m.logScroll = 0
	case "G":
		m.logScroll = max(0, count-1)
	case "x", "c":
		m.logs.Clear()
		m.logScroll = 0
	case "esc", "q":
		m.mode = ModeNormal
	}
	return m, nil
}

// renderLogs shows buffered log entries newest-first, most recent at top.
func (m Model) renderLogs() string {
	entries := m.logs.Entries()

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(
		fmt.Sprintf("  Logs (%d/%d)  ", len(entries), applog.Capacity),
	))
	rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render(
		"  j/k: scroll  g/G: top/bottom  x/c: clear  esc: close",
	))
	rows = append(rows, "")

	if len(entries) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  No log entries yet."))
	} else {
		start := m.logScroll
		end := min(start+logsVisibleRows, len(entries))
		for i := end - 1; i >= start; i-- {
			rows = append(rows, renderLogEntry(entries[i]))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return DialogStyle.Width(100).Render(content)
}

func renderLogEntry(e applog.Entry) string {
	levelStyle := lipgloss.NewStyle().Foreground(SubtleColor)
	if e.Level == applog.LevelError {
		levelStyle = lipgloss.NewStyle().Foreground(WarningColor)
	}

	timestamp := lipgloss.NewStyle().Foreground(SubtleColor).Render(e.Time.Format("15:04:05"))
	level := levelStyle.Bold(true).Render(fmt.Sprintf("%-5s", e.Level))
	message := strings.ReplaceAll(e.Message, "\n", " ")

	return fmt.Sprintf("  %s  %s  %s", timestamp, level, message)
}
