package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/azuredevops"
)

const commentsVisibleRows = 15

func (m Model) handleCommentsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	count := len(m.comments)

	switch msg.String() {
	case "j", "down":
		if m.commentsScroll < count-1 {
			m.commentsScroll++
		}
	case "k", "up":
		if m.commentsScroll > 0 {
			m.commentsScroll--
		}
	case "g":
		m.commentsScroll = 0
	case "G":
		m.commentsScroll = max(0, count-1)
	case "esc", "q":
		m.mode = ModeNormal
	}
	return m, nil
}

// renderComments shows a work item's comments oldest-first.
func (m Model) renderComments() string {
	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(
		fmt.Sprintf("  Comments — %s  ", m.commentsTaskTitle),
	))
	rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render(
		"  j/k: scroll  g/G: top/bottom  esc: close",
	))
	rows = append(rows, "")

	switch {
	case m.commentsLoading:
		rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  Loading comments..."))
	case len(m.comments) == 0:
		rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  No comments yet."))
	default:
		end := min(m.commentsScroll+commentsVisibleRows, len(m.comments))
		for i := m.commentsScroll; i < end; i++ {
			rows = append(rows, renderComment(m.comments[i]), "")
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return DialogStyle.Width(100).Render(content)
}

func renderComment(c azuredevops.Comment) string {
	author := lipgloss.NewStyle().Foreground(HighlightColor).Bold(true).Render(c.CreatedBy.DisplayName)
	timestamp := lipgloss.NewStyle().Foreground(SubtleColor).Render(formatCommentDate(c.CreatedDate))
	lines := []string{fmt.Sprintf("  %s  %s", author, timestamp)}

	for line := range strings.SplitSeq(strings.TrimSpace(c.Text), "\n") {
		lines = append(lines, "    "+line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatCommentDate(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Local().Format("2006-01-02 15:04")
}
