package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"cmdban/internal/config"
)

const (
	topBarSectionRows   = 7 // rows under each section heading
	topBarTitleRows     = 1
	topBarHeadingRows   = 1
	topBarBorderRows    = 1 // the rule under the bar
	topBarPaddingCols   = 2
	topBarNameMax       = 30
	topBarSeparator     = " │ "
	topBarShortcutsGap  = 3
	activeMarker        = "● "
	inactiveMarker      = "○ "
	topBarNoneLabel     = "none"
	topBarTitle         = "cmdban"
	syncedJustNowWindow = time.Second

	// topBarHeight is the rendered height; renderTopBar truncates lines so
	// they never wrap and grow it.
	topBarHeight = topBarTitleRows + topBarHeadingRows + topBarSectionRows + topBarBorderRows
)

var (
	topBarStyle = lipgloss.NewStyle().
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(BorderColor)
	topBarTitleStyle     = lipgloss.NewStyle().Foreground(ActiveColor).Bold(true)
	topBarHeadingStyle   = lipgloss.NewStyle().Foreground(SubtleColor).Bold(true)
	topBarSeparatorStyle = lipgloss.NewStyle().Foreground(BorderColor)
	topBarLabelStyle     = lipgloss.NewStyle().Foreground(ActiveColor)
	topBarValueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	topBarDimStyle       = lipgloss.NewStyle().Foreground(SubtleColor)
	topBarActiveStyle    = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	topBarKeyStyle       = lipgloss.NewStyle().Foreground(HighlightColor).Bold(true)
)

type shortcut struct {
	key  string
	desc string
}

// renderTopBar lays the board context, the work item type filter (ADO only)
// and the current mode's shortcuts out as headed sections divided by
// vertical rules.
func (m Model) renderTopBar() string {
	sections := [][]string{
		topBarSection("CONTEXT", m.buildContextInfo()),
		topBarSection("RECENT", m.buildRecentBoardsSection()),
	}
	if m.config.IsADOBoard() {
		sections = append(sections, topBarSection("TYPES", m.buildTypeFilterSection()))
	}
	used := topBarPaddingCols
	for _, section := range sections {
		used += columnWidth(section) + lipgloss.Width(topBarSeparator)
	}
	sections = append(sections, topBarSection("SHORTCUTS", m.buildShortcutsDisplay(m.width-used)))

	title := topBarTitleStyle.Render(topBarTitle) + "  " + topBarDimStyle.Render(m.modeName())
	lines := strings.Split(title+"\n"+joinTopBarSections(sections), "\n")
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(m.width-topBarPaddingCols, 0), "")
	}
	for len(lines) < topBarTitleRows+topBarHeadingRows+topBarSectionRows {
		lines = append(lines, "")
	}
	return topBarStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

func topBarSection(heading string, lines []string) []string {
	return append([]string{topBarHeadingStyle.Render(heading)}, lines...)
}

func columnWidth(lines []string) int {
	widest := 0
	for _, line := range lines {
		widest = max(widest, lipgloss.Width(line))
	}
	return widest
}

// joinTopBarSections pads every section to its widest line and puts a
// vertical rule between them on every row.
func joinTopBarSections(sections [][]string) string {
	rows := 0
	for _, section := range sections {
		rows = max(rows, len(section))
	}

	separator := topBarSeparatorStyle.Render(topBarSeparator)
	lines := make([]string, rows)
	for i := range lines {
		var cells []string
		for s, section := range sections {
			cell := ""
			if i < len(section) {
				cell = section[i]
			}
			if s < len(sections)-1 {
				cell += strings.Repeat(" ", columnWidth(section)-lipgloss.Width(cell))
			}
			cells = append(cells, cell)
		}
		lines[i] = strings.TrimRight(strings.Join(cells, separator), " ")
	}
	return strings.Join(lines, "\n")
}

func truncateName(name string) string {
	return ansi.Truncate(name, topBarNameMax, "…")
}

func topBarLine(icon, label, value string) string {
	return icon + " " + topBarLabelStyle.Render(label+": ") + value
}

func (m Model) buildContextInfo() []string {
	board := m.config.ActiveBoard()
	if board == nil {
		return []string{topBarLine("📋", "Board", topBarDimStyle.Render(topBarNoneLabel))}
	}

	boardName := topBarValueStyle.Render(truncateName(board.Name))
	if board.Type == config.BoardTypeAzureDevOps {
		boardName += topBarDimStyle.Render(" [ADO]")
	}
	lines := []string{topBarLine("📋", "Board", boardName)}

	if m.config.IsADOBoard() {
		ado := board.AzureDevOps
		lines = append(lines,
			topBarLine("🔗", "Source", topBarValueStyle.Render(truncateName(ado.Org+"/"+ado.Project+"/"+ado.Team))),
			topBarLine("🧱", "Lanes", topBarValueStyle.Render(m.laneModeDescription(ado))),
			topBarLine("🔄", "Synced", topBarValueStyle.Render(m.syncedDescription())),
		)
	} else {
		lines = append(lines, topBarLine("📁", "Dir", topBarValueStyle.Render(truncateName(m.config.TaskDirectory()))))
	}

	lines = append(lines, topBarLine("📝", "Tasks", topBarValueStyle.Render(m.taskCountDescription())))
	if m.config.IsADOBoard() {
		lines = append(lines, topBarLine("💬", "Comments", m.commentCountDescription()))
	}
	lines = append(lines, topBarLine("🔎", "Filter", m.filterDescription()))
	return lines
}

func (m Model) laneModeDescription(ado *config.AzureDevOpsConfig) string {
	if !ado.UsesBoardColumns() {
		return "states (legacy)"
	}
	label := laneModeLabel(ado.ShowsStateLanes())
	if ado.BoardLevel != "" {
		label += " · " + ado.BoardLevel
	}
	return label
}

func (m Model) syncedDescription() string {
	if m.lastSyncTime.IsZero() {
		return "never"
	}
	elapsed := time.Since(m.lastSyncTime).Round(time.Second)
	if elapsed < syncedJustNowWindow {
		return "just now"
	}
	return elapsed.String() + " ago"
}

func (m Model) taskCountDescription() string {
	total, shown := 0, 0
	for _, lane := range m.board.Lanes {
		total += len(m.board.Tasks[lane])
		shown += len(m.filterTasks(m.board.Tasks[lane]))
	}
	if shown == total {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d of %d", shown, total)
}

// commentCountDescription reports the selected card's comment count from
// the last sync.
func (m Model) commentCountDescription() string {
	t := m.selectedTask()
	if t == nil {
		return topBarDimStyle.Render(topBarNoneLabel)
	}
	return topBarValueStyle.Render(fmt.Sprintf("%d", t.ADOCommentCount)) + topBarDimStyle.Render(fmt.Sprintf(" on #%d (c)", t.ADOItemID))
}

func (m Model) filterDescription() string {
	var parts []string
	if m.typeFilter != "" {
		parts = append(parts, "type:"+m.typeFilter)
	}
	if m.searchFilter != "" {
		parts = append(parts, "/"+m.searchFilter)
	}
	if len(parts) == 0 {
		return topBarDimStyle.Render(topBarNoneLabel)
	}
	return topBarActiveStyle.Render(truncateName(strings.Join(parts, " "))) + topBarDimStyle.Render(" (esc)")
}

// buildRecentBoardsSection lists the most recently used boards, marking the
// active one; ctrl+n cycles through them in this order.
func (m Model) buildRecentBoardsSection() []string {
	current := ""
	if board := m.config.ActiveBoard(); board != nil {
		current = board.Name
	}
	var lines []string
	for _, name := range m.config.RecentBoardNames() {
		label := truncateName(name)
		if board := m.config.BoardByName(name); board != nil && board.Type == config.BoardTypeAzureDevOps {
			label += topBarDimStyle.Render(" [ADO]")
		}
		if name == current {
			lines = append(lines, topBarActiveStyle.Render(activeMarker)+topBarActiveStyle.Render(label))
		} else {
			lines = append(lines, topBarValueStyle.Render(inactiveMarker)+topBarValueStyle.Render(label))
		}
	}
	return append(lines, topBarDimStyle.Render("<ctrl+n> next"))
}

// buildTypeFilterSection lists the number key for every work item type on
// the board, marking the active filter, with a "+N more" row when they
// don't all fit.
func (m Model) buildTypeFilterSection() []string {
	types := m.workItemTypes()
	lines := []string{m.typeFilterLine(typeFilterAll, "all", m.typeFilter == "")}

	shown := types
	if len(types) > topBarSectionRows-1 {
		shown = types[:topBarSectionRows-2]
	}
	for i, workItemType := range shown {
		label := fmt.Sprintf("%s (%d)", workItemType, m.workItemTypeCount(workItemType))
		lines = append(lines, m.typeFilterLine(i+1, label, m.typeFilter == workItemType))
	}
	if len(shown) < len(types) {
		lines = append(lines, topBarDimStyle.Render(fmt.Sprintf("+%d more", len(types)-len(shown))))
	}
	return lines
}

func (m Model) typeFilterLine(index int, label string, active bool) string {
	keyLabel := topBarKeyStyle.Render(fmt.Sprintf("<%d>", index))
	if active {
		return keyLabel + " " + topBarActiveStyle.Render(activeMarker+label)
	}
	return keyLabel + " " + topBarValueStyle.Render(inactiveMarker+label)
}

// buildShortcutsDisplay flows the mode's shortcuts into as many columns of
// topBarSectionRows as fit in width, dropping the ones that don't fit.
func (m Model) buildShortcutsDisplay(width int) []string {
	shortcuts := m.contextualShortcuts()
	formatted := make([]string, len(shortcuts))
	for i, sc := range shortcuts {
		formatted[i] = topBarKeyStyle.Render("<"+sc.key+">") + " " + topBarDimStyle.Render(sc.desc)
	}

	rows := make([]string, min(len(formatted), topBarSectionRows))
	used := 0
	for start := 0; start < len(formatted); start += topBarSectionRows {
		column := formatted[start:min(start+topBarSectionRows, len(formatted))]
		columnW := columnWidth(column)
		gap := 0
		if start > 0 {
			gap = topBarShortcutsGap
		}
		if used+gap+columnW > width {
			break
		}
		for i := range column {
			rows[i] += strings.Repeat(" ", used+gap-lipgloss.Width(rows[i])) + column[i]
		}
		used += gap + columnW
	}
	return rows
}

func (m Model) contextualShortcuts() []shortcut {
	if m.mode != ModeNormal {
		return shortcutsForMode(m.mode)
	}
	if m.config.IsADOBoard() {
		return slices.Concat(boardNavigationShortcuts, adoBoardShortcuts, boardShortcuts, []shortcut{quitShortcut})
	}
	return slices.Concat(boardNavigationShortcuts, boardShortcuts, []shortcut{quitShortcut})
}

var (
	quitShortcut = shortcut{"q", "quit"}
	// boardNavigationShortcuts lead the topbar so they survive truncation on
	// narrow terminals, followed by ADO shortcuts, then the rest.
	boardNavigationShortcuts = []shortcut{
		{"h/l", "lane"}, {"j/k", "task"}, {"H/L", "move lane"}, {"enter", "view"}, {"tab", "focus"}, {"a", "new"},
	}
	boardShortcuts = []shortcut{
		{"e", "edit"}, {"d", "toggle done"}, {"x", "delete"}, {"t", "tag"}, {"m", "mark"}, {"/", "search"},
		{"r", "refresh"}, {".", "minimize lane"}, {"ctrl+b", "boards"}, {"ctrl+n", "next recent board"},
		{"ctrl+m", "move board"}, {":", "command"}, {"?", "help"},
	}
	adoBoardShortcuts = []shortcut{
		{"0-9", "type filter"}, {"v", "column/state lanes"}, {"c", "comments"}, {"ctrl+o", "open url"},
	}
)

// shortcutsForMode lists the keys available in a non-board mode. The board
// (normal) mode is assembled by contextualShortcuts since it depends on the
// board type.
func shortcutsForMode(mode Mode) []shortcut {
	switch mode {
	case ModeInput, ModeCommand, ModeADOSetup:
		return []shortcut{{"enter", "confirm"}, {"esc", "cancel"}, {"j/k", "navigate lists"}}
	case ModeTag:
		return []shortcut{{"enter", "add tag"}, {"tab", "complete"}, {"esc", "cancel"}}
	case ModeSearch:
		return []shortcut{{"enter", "apply"}, {"esc", "clear"}}
	case ModeConfirm:
		return []shortcut{{"y", "yes"}, {"n/esc", "no"}}
	case ModeHelp:
		return []shortcut{{"j/k", "scroll"}, {"g/G", "top/bottom"}, {"?/q", "close help"}}
	case ModeView:
		return []shortcut{
			{"q", "back"}, {"j/k", "scroll"}, {"tab", "comments pane"}, {"J/K", "scroll comments"}, {"c", "comments view"},
			{"H/L", "move lane"}, {"e", "edit"}, {"d", "toggle done"}, {"x", "delete"},
			{"ctrl+g", "editor"}, {"ctrl+o", "open url"}, {"ctrl+m", "move board"}, {"r", "refresh"},
		}
	case ModeFocus:
		return []shortcut{
			{"tab/q", "exit focus"}, {"j/k", "select"}, {"h/l", "lane"}, {"H/L", "move lane"}, {"a", "new"}, {"e", "edit"},
			{"t", "tag"}, {"m", "mark"}, {"c", "comments"}, {"ctrl+o", "open url"}, {"r", "refresh"},
		}
	case ModeBoardSelector:
		return []shortcut{{"j/k", "navigate"}, {"enter", "switch"}, {"a", "new local board"}, {"A", "new ADO board"}, {"q", "close"}}
	case ModeMoveBoard:
		return []shortcut{{"j/k", "navigate"}, {"enter", "move"}, {"q", "cancel"}}
	case ModePATManager:
		return []shortcut{{"j/k", "navigate"}, {"a", "add"}, {"x", "delete"}, {"space", "reveal"}, {"q", "close"}}
	case ModeLogs:
		return []shortcut{{"j/k", "scroll"}, {"g/G", "top/bottom"}, {"x", "clear"}, {"q", "close"}}
	case ModeComments:
		return []shortcut{{"j/k", "scroll"}, {"g/G", "top/bottom"}, {"q", "close"}}
	default:
		return nil
	}
}

func (m Model) modeName() string {
	switch m.mode {
	case ModeInput:
		return "INSERT"
	case ModeCommand:
		return "COMMAND"
	case ModeConfirm:
		return "CONFIRM"
	case ModeHelp:
		return "HELP"
	case ModeView:
		return "VIEW"
	case ModeTag:
		return "TAG"
	case ModeSearch:
		return "SEARCH"
	case ModeBoardSelector:
		return "BOARDS"
	case ModePATManager:
		return "PATS"
	case ModeADOSetup:
		return "ADO SETUP"
	case ModeLogs:
		return "LOGS"
	case ModeComments:
		return "COMMENTS"
	case ModeFocus:
		return "FOCUS"
	case ModeMoveBoard:
		return "MOVE"
	default:
		return "NORMAL"
	}
}
