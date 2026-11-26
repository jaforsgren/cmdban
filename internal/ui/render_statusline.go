package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	statusLineSeparator     = " │ "
	statusLineClockInterval = time.Second
	commandLinePrefix       = ":"
	searchLinePrefix        = "/"
)

var (
	statusLineModeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ActiveColor).
				Bold(true).
				Padding(0, 1)
	statusLineSegmentStyle = lipgloss.NewStyle().Foreground(SubtleColor)
	statusLineMessageStyle = lipgloss.NewStyle().Foreground(WarningColor)
	statusLineLiveStyle    = topBarLiveReloadStyle
	commandLineStyle       = lipgloss.NewStyle().Padding(0, 1)
)

// statusClockMsg re-renders the status line so its "synced … ago" keeps
// counting while no other message arrives.
type statusClockMsg struct{}

func scheduleStatusClock() tea.Cmd {
	return tea.Tick(statusLineClockInterval, func(time.Time) tea.Msg {
		return statusClockMsg{}
	})
}

// renderStatusLine is the vim-like bottom row: the mode, the board's
// context, then the latest message. Truncated so it stays one row tall.
func (m Model) renderStatusLine() string {
	separator := statusLineSegmentStyle.Render(statusLineSeparator)
	line := statusLineModeStyle.Render(m.modeName()) + " " + strings.Join(m.statusLineSegments(), separator)
	if m.message != "" {
		line += separator + statusLineMessageStyle.Render(m.message)
	}
	return commandLineStyle.Render(ansi.Truncate(line, max(m.width-footerPaddingCols, 0), "…"))
}

func (m Model) statusLineSegments() []string {
	if isWikiMode(m.displayMode()) {
		return []string{
			statusLineSegmentStyle.Render(fmt.Sprintf("%d wikis", len(m.config.Wikis))),
			statusLineSyncSegment(m.wiki.lastSync, m.wiki.watch),
		}
	}

	var segments []string
	if m.mode == ModeVisual {
		segments = append(segments, fmt.Sprintf("%d selected", len(m.bulkTargets())))
	}
	segments = append(segments, m.taskCountDescription()+" tasks")
	if marked := m.markedTaskCount(); marked > 0 {
		segments = append(segments, fmt.Sprintf("%d marked", marked))
	}
	if m.searchFilter != "" {
		segments = append(segments, searchLinePrefix+m.searchFilter)
	}
	if board := m.config.ActiveBoard(); board != nil {
		segments = append(segments, board.Name)
	}
	for i := range segments {
		segments[i] = statusLineSegmentStyle.Render(segments[i])
	}
	return append(segments, statusLineSyncSegment(m.lastSyncTime, m.liveReload))
}

func statusLineSyncSegment(lastSync time.Time, live bool) string {
	description := "synced " + describeSync(lastSync)
	if live {
		return statusLineLiveStyle.Render(description + " · live")
	}
	return statusLineSegmentStyle.Render(description)
}

func (m Model) markedTaskCount() int {
	marked := 0
	for _, lane := range m.board.Lanes {
		for _, t := range m.board.Tasks[lane] {
			if t.Marked {
				marked++
			}
		}
	}
	return marked
}

// renderCommandLine is the single-row prompt for `:` commands and `/`
// search, typed in place of the status line.
func (m Model) renderCommandLine(prefix string) string {
	input := m.textInput
	input.Prompt = ""
	return commandLineStyle.Render(ansi.Truncate(prefix+input.View(), max(m.width-footerPaddingCols, 0), ""))
}
