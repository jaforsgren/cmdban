package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"cmdban/internal/azuredevops"
	"cmdban/internal/task"
)

const (
	commentsPaneBorderRows  = 2
	commentsPaneBorderCols  = 2
	commentsPanePaddingCols = 2
	commentsHeadingRows     = 2 // heading and the blank line under it
	commentBodyIndent       = "  "
	// commentsSidePaneShare is the fraction of the terminal width the
	// comments pane takes beside a card's details.
	commentsSidePaneShare = 0.4
)

var (
	commentsPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor).
				Padding(0, 1)
	commentsHeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(HighlightColor)
	commentsDimStyle     = lipgloss.NewStyle().Foreground(SubtleColor).Italic(true)
	commentAuthorStyle   = lipgloss.NewStyle().Foreground(HighlightColor).Bold(true)
	commentDateStyle     = lipgloss.NewStyle().Foreground(SubtleColor)
)

// openComments shows the selected work item's comments in their own view;
// closing it returns to the mode it was opened from.
func (m Model) openComments() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	if !m.config.IsADOBoard() {
		m.message = "Comments are only available on Azure DevOps boards"
		return m, nil
	}
	m.commentsReturnMode = m.mode
	m.mode = ModeComments
	return m.startLoadingComments(t)
}

// toggleCommentsPane shows or hides the comments beside a card's details.
func (m Model) toggleCommentsPane() (tea.Model, tea.Cmd) {
	if m.viewCommentsPane {
		m.viewCommentsPane = false
		return m, nil
	}
	t := m.selectedTask()
	if t == nil || t.ADOItemID == 0 {
		m.message = "Comments are only available on Azure DevOps boards"
		return m, nil
	}
	m.viewCommentsPane = true
	return m.startLoadingComments(t)
}

func (m Model) startLoadingComments(t *task.Task) (tea.Model, tea.Cmd) {
	m.comments = nil
	m.commentsScroll = 0
	m.commentsLoading = true
	m.commentsTaskTitle = t.Title
	m.commentsWorkItemID = t.ADOItemID
	return m, m.fetchComments(t.ADOItemID)
}

func (m Model) fetchComments(workItemID int) tea.Cmd {
	adoCfg := m.config.ActiveBoard().AzureDevOps
	return func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		comments, err := azuredevops.FetchComments(adoCfg, token, workItemID)
		if err != nil {
			return errMsg{err}
		}
		return commentsLoadedMsg{workItemID: workItemID, comments: comments}
	}
}

// handleCommentsLoaded drops responses for a work item the user has since
// navigated away from.
func (m Model) handleCommentsLoaded(msg commentsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.workItemID != m.commentsWorkItemID {
		return m, nil
	}
	m.commentsLoading = false
	m.comments = msg.comments
	return m, nil
}

func (m Model) handleCommentsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := m.maxCommentsScroll(m.width, m.bodyHeight())
	switch msg.String() {
	case "j", "down":
		m.commentsScroll = min(m.commentsScroll+1, maxScroll)
	case "k", "up":
		m.commentsScroll = max(m.commentsScroll-1, 0)
	case "g":
		m.commentsScroll = 0
	case "G":
		m.commentsScroll = maxScroll
	case "esc", "q":
		m.mode = m.commentsReturnMode
	}
	return m, nil
}

// scrollCommentsPane scrolls the side pane in the task view by delta lines.
func (m Model) scrollCommentsPane(delta int) (tea.Model, tea.Cmd) {
	if !m.viewCommentsPane {
		return m, nil
	}
	maxScroll := m.maxCommentsScroll(m.commentsSidePaneWidth(), m.bodyHeight())
	m.commentsScroll = min(max(m.commentsScroll+delta, 0), maxScroll)
	return m, nil
}

func (m Model) renderComments() string {
	return m.renderCommentsPane(m.width, m.bodyHeight())
}

// renderCommentsPane renders the loaded comments in a bordered box of the
// given outer size, scrolled by commentsScroll lines.
func (m Model) renderCommentsPane(width, height int) string {
	visibleRows := commentsVisibleRows(height)
	lines := m.commentLines(commentsTextWidth(width))
	start := min(m.commentsScroll, max(len(lines)-visibleRows, 0))
	end := min(start+visibleRows, len(lines))

	heading := commentsHeadingStyle.Render(fmt.Sprintf("Comments (%d) — %s", len(m.comments), m.commentsTaskTitle))
	body := strings.Join(lines[start:end], "\n")
	return commentsPaneStyle.
		Width(width - commentsPaneBorderCols).
		Height(height - commentsPaneBorderRows).
		Render(truncateLine(heading, commentsTextWidth(width)) + "\n\n" + body)
}

func commentsVisibleRows(height int) int {
	return max(height-commentsPaneBorderRows-commentsHeadingRows, 1)
}

func commentsTextWidth(width int) int {
	return max(width-commentsPaneBorderCols-commentsPanePaddingCols, 1)
}

func (m Model) maxCommentsScroll(width, height int) int {
	return max(len(m.commentLines(commentsTextWidth(width)))-commentsVisibleRows(height), 0)
}

// commentLines renders every comment oldest-first, wrapped to width, with a
// blank line between comments.
func (m Model) commentLines(width int) []string {
	switch {
	case m.commentsLoading:
		return []string{commentsDimStyle.Render("Loading comments...")}
	case len(m.comments) == 0:
		return []string{commentsDimStyle.Render("No comments yet.")}
	}

	var lines []string
	bodyStyle := lipgloss.NewStyle().Width(max(width-len(commentBodyIndent), 1))
	for i, c := range m.comments {
		if i > 0 {
			lines = append(lines, "")
		}
		header := commentAuthorStyle.Render(c.CreatedBy.DisplayName) + "  " + commentDateStyle.Render(formatCommentDate(c.CreatedDate))
		lines = append(lines, truncateLine(header, width))
		for line := range strings.SplitSeq(bodyStyle.Render(strings.TrimSpace(c.Text)), "\n") {
			lines = append(lines, commentBodyIndent+line)
		}
	}
	return lines
}

func (m Model) commentsSidePaneWidth() int {
	return int(float64(m.width) * commentsSidePaneShare)
}

func truncateLine(s string, width int) string {
	return ansi.Truncate(s, width, "…")
}

func formatCommentDate(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Local().Format("2006-01-02 15:04")
}
