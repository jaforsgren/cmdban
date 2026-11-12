package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	helpTitle         = "Keyboard Shortcuts & Commands — All Contexts"
	helpCommandsTitle = "Commands"
	helpDividerWidth  = 44
	helpKeyIndent     = "  "
	helpKeyDescGap    = "  "
	helpColumnGap     = 4
	helpMaxColumns    = 4
)

var (
	helpHeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveColor)
	helpKeyStyle     = lipgloss.NewStyle().Bold(true).Foreground(HighlightColor)
	helpDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpDimStyle     = lipgloss.NewStyle().Foreground(SubtleColor)
)

type helpSection struct {
	title     string
	shortcuts []shortcut
}

// commandHelp documents every : command handled by handleCommand.
var commandHelp = []shortcut{
	{":adocolumns [board]", "sync lanes from an ADO board"},
	{":boards / :board / :b", "board switcher"},
	{":clear", "archive all tasks in the lane"},
	{":config / :conf / :c", "open config in $EDITOR"},
	{":help / :h", "this help"},
	{":logs / :log / :l", "error/log viewer"},
	{":move / :m", "move task to another board"},
	{":pats / :p", "PAT manager"},
	{":quit / :q", "quit"},
	{":settings dir <path>", "set the board's task directory"},
	{":write / :w", "tasks auto-save (no-op)"},
}

// generalShortcuts work the same in every view, so the per-view help
// sections leave them out.
var generalShortcuts = []shortcut{
	{taskNavigationKeys, "move / scroll"},
	{laneNavigationKeys, "previous / next lane"},
	{topBottomKeys, "top / bottom"},
	{closeKey, "close / back (quits on the board)"},
	{"esc", "cancel"},
	{"?", "help"},
}

// helpSections lists the general keys, the board's own keys and commands,
// then each view's keys minus the general ones and those the board already
// lists.
// Sections left empty are dropped.
func helpSections() []helpSection {
	board := withoutGeneralShortcuts(slices.Concat(boardNavigationShortcuts, boardShortcuts))
	sections := []helpSection{
		{"General", generalShortcuts},
		{"Board", board},
		{"Azure DevOps boards", adoBoardShortcuts},
		{helpCommandsTitle, commandHelp},
	}
	known := slices.Concat(board, adoBoardShortcuts)
	for _, view := range []struct {
		title string
		mode  Mode
	}{
		{"Task view", ModeView},
		{"Focus", ModeFocus},
		{"Comments", ModeComments},
		{"Board switcher", ModeBoardSelector},
		{"Move to board", ModeMoveBoard},
		{"Link PR", ModePRPicker},
		{"PAT manager", ModePATManager},
		{"Logs", ModeLogs},
		{"Input / command / ADO setup", ModeInput},
		{"Search", ModeSearch},
		{"Tag", ModeTag},
		{"Confirm", ModeConfirm},
	} {
		shortcuts := slices.DeleteFunc(withoutGeneralShortcuts(shortcutsForMode(view.mode)), func(sc shortcut) bool {
			return slices.Contains(known, sc)
		})
		if len(shortcuts) > 0 {
			sections = append(sections, helpSection{view.title, shortcuts})
		}
	}
	return sections
}

func withoutGeneralShortcuts(shortcuts []shortcut) []shortcut {
	return slices.DeleteFunc(slices.Clone(shortcuts), func(sc shortcut) bool {
		return slices.ContainsFunc(generalShortcuts, func(general shortcut) bool { return general.key == sc.key })
	})
}

func renderHelpSection(title string, entries []shortcut) string {
	keyWidth := 0
	for _, entry := range entries {
		keyWidth = max(keyWidth, lipgloss.Width(entry.key))
	}

	lines := []string{
		helpHeadingStyle.Render(title),
		helpDimStyle.Render(strings.Repeat("─", helpDividerWidth)),
	}
	for _, entry := range entries {
		key := helpKeyStyle.Render(fmt.Sprintf("%-*s", keyWidth, entry.key))
		lines = append(lines, helpKeyIndent+key+helpKeyDescGap+helpDescStyle.Render(entry.desc))
	}
	return strings.Join(lines, "\n")
}

func (m Model) helpContentLines() []string {
	var blocks []string
	for _, section := range helpSections() {
		blocks = append(blocks, renderHelpSection(section.title, section.shortcuts))
	}

	columns := m.fitHelpColumns(blocks)
	rendered := make([]string, len(columns))
	for i, column := range columns {
		rendered[i] = strings.Join(column, "\n\n")
		if i < len(columns)-1 {
			rendered[i] = lipgloss.NewStyle().PaddingRight(helpColumnGap).Render(rendered[i])
		}
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	return strings.Split(helpHeadingStyle.Render(helpTitle)+"\n\n"+body, "\n")
}

// fitHelpColumns uses the most columns, up to helpMaxColumns, whose
// balanced layout fits the terminal width.
func (m Model) fitHelpColumns(blocks []string) [][]string {
	for count := min(helpMaxColumns, len(blocks)); count > 1; count-- {
		columns := balanceHelpColumns(blocks, count)
		if helpColumnsWidth(columns) <= m.width {
			return columns
		}
	}
	return balanceHelpColumns(blocks, 1)
}

func helpColumnsWidth(columns [][]string) int {
	width := helpColumnGap * (len(columns) - 1)
	for _, column := range columns {
		width += columnWidth(column)
	}
	return width
}

// balanceHelpColumns keeps the sections in order and picks the column
// breaks that make the tallest column as short as possible.
func balanceHelpColumns(blocks []string, columnCount int) [][]string {
	heights := make([]int, len(blocks))
	tallest, total := 0, 0
	for i, block := range blocks {
		heights[i] = lipgloss.Height(block) + 1 // blank line between sections
		tallest = max(tallest, heights[i])
		total += heights[i]
	}
	for limit := tallest; limit < total; limit++ {
		if columns := packHelpColumns(blocks, heights, limit); len(columns) <= columnCount {
			return columns
		}
	}
	return [][]string{blocks}
}

// packHelpColumns fills each column with sections until the next one would
// make it taller than limit.
func packHelpColumns(blocks []string, heights []int, limit int) [][]string {
	columns := [][]string{nil}
	height := 0
	for i, block := range blocks {
		if height > 0 && height+heights[i] > limit {
			columns = append(columns, nil)
			height = 0
		}
		columns[len(columns)-1] = append(columns[len(columns)-1], block)
		height += heights[i]
	}
	return columns
}

// renderHelp shows the scrolled window of the help content that fits the
// body area.
func (m Model) renderHelp() string {
	lines := m.helpContentLines()
	height := m.bodyHeight()
	start := min(m.viewScroll, max(len(lines)-height, 0))
	end := min(start+height, len(lines))
	return lipgloss.NewStyle().Padding(0, 1).Render(strings.Join(lines[start:end], "\n"))
}

// maxHelpScroll is the furthest the help view can scroll before its last
// line reaches the bottom of the body area.
func (m Model) maxHelpScroll() int {
	return max(len(m.helpContentLines())-m.bodyHeight(), 0)
}

func (m Model) openHelp() (tea.Model, tea.Cmd) {
	m.mode = ModeHelp
	m.viewScroll = 0
	return m, nil
}

func (m Model) handleHelpMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Escape), msg.String() == "q":
		m.mode = ModeNormal
	case key.Matches(msg, m.keys.Down):
		m.viewScroll = min(m.viewScroll+1, m.maxHelpScroll())
	case key.Matches(msg, m.keys.Up):
		m.viewScroll = max(m.viewScroll-1, 0)
	case key.Matches(msg, m.keys.Top):
		m.viewScroll = 0
	case key.Matches(msg, m.keys.Bottom):
		m.viewScroll = m.maxHelpScroll()
	}
	return m, nil
}
