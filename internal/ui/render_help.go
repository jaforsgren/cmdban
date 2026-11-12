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
	// Below this terminal width the commands column is stacked under the
	// shortcuts instead of beside them.
	helpTwoColumnMinWidth = 110
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
	{":adocolumns [board]", "sync lanes from an ADO Kanban board"},
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

func helpSections() []helpSection {
	return []helpSection{
		{"Board", slices.Concat(boardNavigationShortcuts, boardShortcuts, []shortcut{quitShortcut})},
		{"Azure DevOps boards", adoBoardShortcuts},
		{"Task view", shortcutsForMode(ModeView)},
		{"Focus", shortcutsForMode(ModeFocus)},
		{"Board switcher", shortcutsForMode(ModeBoardSelector)},
		{"Move to board", shortcutsForMode(ModeMoveBoard)},
		{"PAT manager", shortcutsForMode(ModePATManager)},
		{"Logs", shortcutsForMode(ModeLogs)},
		{"Comments", shortcutsForMode(ModeComments)},
		{"Input / command / ADO setup", shortcutsForMode(ModeInput)},
		{"Search", shortcutsForMode(ModeSearch)},
		{"Tag", shortcutsForMode(ModeTag)},
		{"Confirm", shortcutsForMode(ModeConfirm)},
		{"Help", shortcutsForMode(ModeHelp)},
	}
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
	var shortcutSections []string
	for _, section := range helpSections() {
		shortcutSections = append(shortcutSections, renderHelpSection(section.title, section.shortcuts))
	}
	shortcuts := strings.Join(shortcutSections, "\n\n")
	commands := renderHelpSection(helpCommandsTitle, commandHelp)

	var body string
	if m.width >= helpTwoColumnMinWidth {
		body = lipgloss.JoinHorizontal(lipgloss.Top, shortcuts, strings.Repeat(" ", helpColumnGap), commands)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, shortcuts, "", commands)
	}
	return strings.Split(helpHeadingStyle.Render(helpTitle)+"\n\n"+body, "\n")
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
