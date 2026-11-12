package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	topBarTitleStyle      = lipgloss.NewStyle().Foreground(ActiveColor).Bold(true)
	topBarHeadingStyle    = lipgloss.NewStyle().Foreground(SubtleColor).Bold(true)
	topBarSeparatorStyle  = lipgloss.NewStyle().Foreground(BorderColor)
	topBarLabelStyle      = lipgloss.NewStyle().Foreground(ActiveColor)
	topBarValueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	topBarDimStyle        = lipgloss.NewStyle().Foreground(SubtleColor)
	topBarActiveStyle     = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	topBarKeyStyle        = lipgloss.NewStyle().Foreground(HighlightColor).Bold(true)
	topBarLiveReloadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a05a5a"))
)

func (m Model) visibleTopBarHeight() int {
	if m.config.HideTopBar {
		return 0
	}
	return topBarHeight
}

// toggleTopBar shows or hides the topbar, remembering the choice in the
// config so it survives restarts.
func (m Model) toggleTopBar() (tea.Model, tea.Cmd) {
	m.config.HideTopBar = !m.config.HideTopBar
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
	}
	return m, nil
}

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
		sections = append(sections, topBarSection("FILTERS", m.buildFilterSection()))
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
		)
	} else {
		lines = append(lines, topBarLine("📁", "Dir", topBarValueStyle.Render(truncateName(m.config.TaskDirectory()))))
	}
	lines = append(lines, m.syncedLine())

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

// syncedLine turns a dull red while live reload is on.
func (m Model) syncedLine() string {
	if m.liveReload {
		return "🔄 " + topBarLiveReloadStyle.Render("Synced: "+m.syncedDescription()+" · live")
	}
	return topBarLine("🔄", "Synced", topBarValueStyle.Render(m.syncedDescription()))
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
	switch m.typeFilter.kind {
	case typeFilterSingle:
		parts = append(parts, "type:"+m.typeFilter.workItemType)
	case typeFilterAllTypes:
		parts = append(parts, "type:all")
	case typeFilterUnreadComments:
		parts = append(parts, "unread comments")
	}
	if m.searchFilter != "" {
		parts = append(parts, "/"+m.searchFilter)
	}
	if m.config.IsADOBoard() && m.config.ActiveBoard().AzureDevOps.IncludeUnassigned {
		parts = append(parts, "+unassigned")
	}
	if len(parts) == 0 {
		return topBarDimStyle.Render(topBarNoneLabel)
	}
	description := topBarActiveStyle.Render(truncateName(strings.Join(parts, " ")))
	if m.hasActiveFilter() {
		description += topBarDimStyle.Render(" (esc)")
	}
	return description
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

// The FILTERS section marks the active filter with an arrow rather than the
// ●/○ markers used elsewhere, which would read as the Bug glyph.
const (
	typeFilterActiveMarker   = "▸ "
	typeFilterInactiveMarker = "  "
)

// buildFilterSection lays the number keys out in two columns: the default
// view and every work item type (with its glyph as a legend), then the
// unassigned toggle, unread comments, all types and a "+N more" row for
// types beyond the number keys. The active filter is marked.
func (m Model) buildFilterSection() []string {
	defaultGlyphs := ""
	for _, workItemType := range defaultViewTypes {
		defaultGlyphs += renderTypeGlyph(workItemType)
	}
	typeColumn := []string{m.typeFilterLine(typeFilterKeyDefault, defaultGlyphs, "stories & bugs", m.typeFilter.kind == typeFilterDefault)}

	types := m.workItemTypes()
	shown := types[:min(len(types), maxTypeFilterTypes)]
	for i, workItemType := range shown {
		label := fmt.Sprintf("%s (%d)", workItemType, m.workItemTypeCount(workItemType))
		active := m.typeFilter.kind == typeFilterSingle && m.typeFilter.workItemType == workItemType
		typeColumn = append(typeColumn, m.typeFilterLine(i+1, renderTypeGlyph(workItemType), label, active))
	}

	var viewColumn []string
	if adoCfg := m.config.ActiveBoard().AzureDevOps; adoCfg.AssignedToMe {
		viewColumn = append(viewColumn, m.typeFilterLine(typeFilterKeyUnassigned, "", "unassigned", adoCfg.IncludeUnassigned))
	}
	unreadLabel := fmt.Sprintf("unread (%d)", m.unreadCommentsTaskCount())
	viewColumn = append(viewColumn,
		m.typeFilterLine(typeFilterKeyUnreadComments, strings.TrimSpace(unreadCommentsMarker), unreadLabel, m.typeFilter.kind == typeFilterUnreadComments),
		m.typeFilterLine(typeFilterKeyAllTypes, "", "all types", m.typeFilter.kind == typeFilterAllTypes),
	)
	if len(shown) < len(types) {
		viewColumn = append(viewColumn, topBarDimStyle.Render(fmt.Sprintf("+%d more", len(types)-len(shown))))
	}
	return sideBySide([][]string{typeColumn, viewColumn}, topBarShortcutsGap)
}

func (m Model) typeFilterLine(index int, glyphs, label string, active bool) string {
	keyLabel := topBarKeyStyle.Render(fmt.Sprintf("<%d>", index))
	if glyphs != "" {
		glyphs += " "
	}
	if active {
		return keyLabel + " " + topBarActiveStyle.Render(typeFilterActiveMarker) + glyphs + topBarActiveStyle.Render(label)
	}
	return keyLabel + " " + typeFilterInactiveMarker + glyphs + topBarValueStyle.Render(label)
}

// buildShortcutsDisplay flows the mode's shortcuts into as many columns of
// topBarSectionRows as fit in width, dropping the ones that don't fit.
func (m Model) buildShortcutsDisplay(width int) []string {
	shortcuts := m.contextualShortcuts()
	formatted := make([]string, len(shortcuts))
	for i, sc := range shortcuts {
		formatted[i] = topBarKeyStyle.Render("<"+sc.key+">") + " " + topBarDimStyle.Render(sc.desc)
	}

	var columns [][]string
	used := 0
	for start := 0; start < len(formatted); start += topBarSectionRows {
		column := formatted[start:min(start+topBarSectionRows, len(formatted))]
		gap := 0
		if start > 0 {
			gap = topBarShortcutsGap
		}
		if used+gap+columnWidth(column) > width {
			break
		}
		columns = append(columns, column)
		used += gap + columnWidth(column)
	}
	return sideBySide(columns, topBarShortcutsGap)
}

// sideBySide joins columns of lines into rows, padding each column to its
// widest line and separating them by gap spaces.
func sideBySide(columns [][]string, gap int) []string {
	var rows []string
	used := 0
	for c, column := range columns {
		if c > 0 {
			used += gap
		}
		for i, line := range column {
			if i == len(rows) {
				rows = append(rows, "")
			}
			rows[i] += strings.Repeat(" ", max(used-lipgloss.Width(rows[i]), 0)) + line
		}
		used += columnWidth(column)
	}
	return rows
}

// contextualShortcuts lists the current mode's keys for the topbar, leaving
// out the ones it has no room to spare for (see topBarOmittedShortcutKeys).
func (m Model) contextualShortcuts() []shortcut {
	shortcuts := shortcutsForMode(m.mode)
	if m.mode == ModeNormal {
		shortcuts = slices.Concat(boardNavigationShortcuts, boardShortcuts, []shortcut{quitShortcut})
		if m.config.IsADOBoard() {
			shortcuts = slices.Concat(boardNavigationShortcuts, adoBoardShortcuts, boardShortcuts, []shortcut{quitShortcut})
		}
	}
	return slices.DeleteFunc(slices.Clone(shortcuts), func(sc shortcut) bool {
		return slices.Contains(topBarOmittedShortcutKeys, sc.key)
	})
}

const (
	laneNavigationKeys = "h/l"
	taskNavigationKeys = "j/k"
	topBottomKeys      = "g/G"
	closeKey           = "q"
	filterKeys         = "0-9"
)

// topBarOmittedShortcutKeys are left out of the topbar but kept in the help:
// navigation is second nature, and the filter keys have their own FILTERS
// section.
var topBarOmittedShortcutKeys = []string{laneNavigationKeys, taskNavigationKeys, filterKeys}

var (
	quitShortcut = shortcut{closeKey, "quit"}
	// boardNavigationShortcuts lead so they survive truncation on narrow
	// terminals, followed by ADO shortcuts, then the rest.
	boardNavigationShortcuts = []shortcut{
		{laneNavigationKeys, "lane"}, {taskNavigationKeys, "task"}, {"H/L", "move lane"}, {"enter", "view"}, {"tab", "focus"}, {"a", "new"},
	}
	boardShortcuts = []shortcut{
		{"e", "edit"}, {"d", "toggle done"}, {"x", "delete"}, {"t", "tag"}, {"m", "mark"}, {"/", "search"},
		{"r", "refresh"}, {"R", "live reload"}, {"T", "toggle topbar"}, {".", "minimize lane"}, {"ctrl+b", "boards"}, {"ctrl+n", "next recent board"},
		{"ctrl+m", "move board"}, {":", "command"}, {"?", "help"},
	}
	adoBoardShortcuts = []shortcut{
		{"b", "new bug"}, {filterKeys, "filters (listed in the topbar)"}, {"U", "unassigned"}, {"v", "column/state lanes"}, {"c", "comments"}, {"ctrl+o", "open card in browser"}, {"O", "open board in browser"}, {"M", "card as markdown"},
	}
)

// shortcutsForMode lists the keys available in a non-board mode. The board
// (normal) mode is assembled by contextualShortcuts since it depends on the
// board type.
func shortcutsForMode(mode Mode) []shortcut {
	switch mode {
	case ModeInput, ModeCommand, ModeADOSetup:
		return []shortcut{{"enter", "confirm"}, {"esc", "cancel"}, {taskNavigationKeys, "navigate lists"}}
	case ModeTag:
		return []shortcut{{"enter", "add tag"}, {"tab", "complete"}, {"esc", "cancel"}}
	case ModeSearch:
		return []shortcut{{"enter", "apply"}, {"esc", "clear"}}
	case ModeConfirm:
		return []shortcut{{"y", "yes"}, {"n/esc", "no"}}
	case ModeHelp:
		return []shortcut{{taskNavigationKeys, "scroll"}, {topBottomKeys, "top/bottom"}, {"?/q", "close help"}}
	case ModeView:
		return []shortcut{
			{closeKey, "back"}, {taskNavigationKeys, "scroll"}, {"tab", "PRs & comments pane"}, {"J/K", "scroll comments"}, {"c", "comments view"}, {"l", "link my PR"},
			{"H/L", "move lane"}, {"e", "edit"}, {"d", "toggle done"}, {"x", "delete"},
			{"ctrl+g", "editor"}, {"M", "card as markdown"}, {"ctrl+o", "open card in browser"}, {"ctrl+m", "move board"}, {"r", "refresh"},
		}
	case ModeFocus:
		return []shortcut{
			{"tab/q", "exit focus"}, {"enter", "open card"}, {taskNavigationKeys, "select"}, {laneNavigationKeys, "lane"}, {"H/L", "move lane"}, {"a", "new"}, {"b", "new bug"}, {"e", "edit"},
			{"t", "tag"}, {"m", "mark"}, {"c", "comments"}, {"M", "card as markdown"}, {"ctrl+o", "open card in browser"}, {"r", "refresh"},
		}
	case ModeBoardSelector:
		return []shortcut{{taskNavigationKeys, "navigate"}, {"enter", "switch"}, {"a", "new local board"}, {"A", "new ADO board"}, {closeKey, "close"}}
	case ModeMoveBoard:
		return []shortcut{{taskNavigationKeys, "navigate"}, {"enter", "move"}, {closeKey, "cancel"}}
	case ModePRPicker:
		return []shortcut{{taskNavigationKeys, "navigate"}, {"enter", "link PR"}, {closeKey, "cancel"}}
	case ModePATManager:
		return []shortcut{{taskNavigationKeys, "navigate"}, {"a", "add"}, {"x", "delete"}, {"space", "reveal"}, {closeKey, "close"}}
	case ModeLogs:
		return []shortcut{{taskNavigationKeys, "scroll"}, {topBottomKeys, "top/bottom"}, {"x", "clear"}, {closeKey, "close"}}
	case ModeComments:
		return []shortcut{{"a", "add comment"}, {taskNavigationKeys, "scroll"}, {topBottomKeys, "top/bottom"}, {closeKey, "close"}}
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
	case ModePRPicker:
		return "LINK PR"
	default:
		return "NORMAL"
	}
}
