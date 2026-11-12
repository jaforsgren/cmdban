package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/azuredevops"
)

const (
	prSectionMaxPRs        = 2
	prDescriptionMaxLines  = 3
	prSectionMaxPipelines  = 3
	minSidePaneCommentRows = 6
	pipelineResultOK       = "succeeded"
	pipelineStatusDone     = "completed"
	prPickerWidth          = 90
)

var (
	prTitleStyle  = lipgloss.NewStyle().Bold(true)
	prDimStyle    = lipgloss.NewStyle().Foreground(SubtleColor)
	prGoodStyle   = lipgloss.NewStyle().Foreground(SuccessColor)
	prBadStyle    = lipgloss.NewStyle().Foreground(WarningColor)
	prPickerTitle = lipgloss.NewStyle().Bold(true).Foreground(HighlightColor)
)

type linkedPRsLoadedMsg struct {
	workItemID int
	prs        []azuredevops.PullRequestSummary
}

type myPRsLoadedMsg struct {
	workItemID int
	prs        []azuredevops.LinkablePullRequest
}

type prLinkedMsg struct {
	workItemID int
	prID       int
}

// prPickerState is the "link one of my PRs" picker opened from a task.
type prPickerState struct {
	workItemID int
	items      []azuredevops.LinkablePullRequest
	index      int
	loading    bool
}

func (m Model) startLoadingLinkedPRs(workItemID int) (Model, tea.Cmd) {
	m.linkedPRs = nil
	m.linkedPRsLoading = true
	m.linkedPRsWorkItemID = workItemID
	adoCfg := m.config.ActiveBoard().AzureDevOps
	return m, func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		prs, err := azuredevops.FetchLinkedPullRequests(adoCfg, token, workItemID)
		if err != nil {
			return errMsg{err}
		}
		return linkedPRsLoadedMsg{workItemID: workItemID, prs: prs}
	}
}

func (m Model) handleLinkedPRsLoaded(msg linkedPRsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.workItemID != m.linkedPRsWorkItemID {
		return m, nil
	}
	m.linkedPRsLoading = false
	m.linkedPRs = msg.prs
	return m, nil
}

// openPRPicker lists the user's own PRs so one can be linked to the
// selected work item.
func (m Model) openPRPicker() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || t.ADOItemID == 0 {
		m.message = "PRs can only be linked to Azure DevOps work items"
		return m, nil
	}
	workItemID := t.ADOItemID
	m.prPicker = prPickerState{workItemID: workItemID, loading: true}
	m.mode = ModePRPicker
	adoCfg := m.config.ActiveBoard().AzureDevOps
	return m, func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		prs, err := azuredevops.FetchMyLinkablePullRequests(adoCfg, token, workItemID)
		if err != nil {
			return errMsg{err}
		}
		return myPRsLoadedMsg{workItemID: workItemID, prs: prs}
	}
}

func (m Model) handleMyPRsLoaded(msg myPRsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.workItemID != m.prPicker.workItemID {
		return m, nil
	}
	m.prPicker.loading = false
	m.prPicker.items = msg.prs
	return m, nil
}

func (m Model) handlePRPickerMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	count := len(m.prPicker.items)
	switch msg.String() {
	case "j", "down":
		if count > 0 {
			m.prPicker.index = (m.prPicker.index + 1) % count
		}
	case "k", "up":
		if count > 0 {
			m.prPicker.index = (m.prPicker.index - 1 + count) % count
		}
	case "enter":
		if count == 0 || m.prPicker.loading {
			return m, nil
		}
		m.mode = ModeView
		return m.linkPR(m.prPicker.workItemID, m.prPicker.items[m.prPicker.index].Ref)
	case "esc", "q":
		m.mode = ModeView
	}
	return m, nil
}

func (m Model) linkPR(workItemID int, ref azuredevops.PullRequestRef) (tea.Model, tea.Cmd) {
	m.message = fmt.Sprintf("Linking PR #%d...", ref.ID)
	adoCfg := m.config.ActiveBoard().AzureDevOps
	return m, func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		if err := azuredevops.LinkPullRequest(adoCfg, token, workItemID, ref); err != nil {
			return errMsg{err}
		}
		return prLinkedMsg{workItemID: workItemID, prID: ref.ID}
	}
}

// handlePRLinked refreshes the side pane's PR section when it is showing the
// work item the PR was linked to.
func (m Model) handlePRLinked(msg prLinkedMsg) (tea.Model, tea.Cmd) {
	m.message = fmt.Sprintf("Linked PR #%d", msg.prID)
	if !m.viewCommentsPane || msg.workItemID != m.linkedPRsWorkItemID {
		return m, nil
	}
	return m.startLoadingLinkedPRs(msg.workItemID)
}

func (m Model) renderPRPicker() string {
	rows := []string{prPickerTitle.Render(fmt.Sprintf("  Link one of your PRs to #%d  ", m.prPicker.workItemID)), ""}
	switch {
	case m.prPicker.loading:
		rows = append(rows, commentsDimStyle.Render("  Loading your pull requests..."))
	case len(m.prPicker.items) == 0:
		rows = append(rows, commentsDimStyle.Render("  No unlinked pull requests created by you."))
	default:
		for i, pr := range m.prPicker.items {
			label := fmt.Sprintf("#%d  %s  %s", pr.Ref.ID, truncateLine(pr.Title, prPickerWidth/2), prDimStyle.Render(pr.Repository+" · "+pr.Status))
			rows = append(rows, renderADOSetupItem(label, i == m.prPicker.index))
		}
	}
	return DialogStyle.Width(prPickerWidth).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderSidePane stacks the linked PRs above the comments beside a card's
// details, giving the comments whatever height remains.
func (m Model) renderSidePane(width, height int) string {
	prs := m.renderPullRequestSection(width)
	commentsHeight := max(height-lipgloss.Height(prs), minSidePaneCommentRows)
	return lipgloss.JoinVertical(lipgloss.Left, prs, m.renderCommentsPane(width, commentsHeight))
}

func (m Model) sidePaneCommentsHeight() int {
	return max(m.bodyHeight()-lipgloss.Height(m.renderPullRequestSection(m.commentsSidePaneWidth())), minSidePaneCommentRows)
}

func (m Model) renderPullRequestSection(width int) string {
	textWidth := commentsTextWidth(width)
	lines := []string{commentsHeadingStyle.Render(fmt.Sprintf("Pull requests (%d)", len(m.linkedPRs))) + prDimStyle.Render("  l: link")}
	switch {
	case m.linkedPRsLoading:
		lines = append(lines, commentsDimStyle.Render("Loading pull requests..."))
	case len(m.linkedPRs) == 0:
		lines = append(lines, commentsDimStyle.Render("No linked pull requests."))
	default:
		shown := m.linkedPRs[:min(len(m.linkedPRs), prSectionMaxPRs)]
		for _, pr := range shown {
			lines = append(lines, "")
			lines = append(lines, pullRequestLines(pr, textWidth)...)
		}
		if hidden := len(m.linkedPRs) - len(shown); hidden > 0 {
			lines = append(lines, prDimStyle.Render(fmt.Sprintf("+%d more", hidden)))
		}
	}
	for i := range lines {
		lines[i] = truncateLine(lines[i], textWidth)
	}
	return commentsPaneStyle.Width(width - commentsPaneBorderCols).Render(strings.Join(lines, "\n"))
}

func pullRequestLines(pr azuredevops.PullRequestSummary, width int) []string {
	lines := []string{
		prTitleStyle.Render(fmt.Sprintf("#%d %s", pr.ID, pr.Title)),
		pullRequestStatusLine(pr),
		prDimStyle.Render(pr.URL),
	}
	if description := strings.TrimSpace(pr.Description); description != "" {
		wrapped := nonBlankLines(lipgloss.NewStyle().Width(width).Render(description))
		for _, line := range wrapped[:min(len(wrapped), prDescriptionMaxLines)] {
			lines = append(lines, prDimStyle.Render(line))
		}
		if len(wrapped) > prDescriptionMaxLines {
			lines = append(lines, prDimStyle.Render("…"))
		}
	}
	for _, run := range pr.Pipelines[:min(len(pr.Pipelines), prSectionMaxPipelines)] {
		lines = append(lines, pipelineRunLine(run), prDimStyle.Render("  "+run.URL))
	}
	return lines
}

func nonBlankLines(s string) []string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func pullRequestStatusLine(pr azuredevops.PullRequestSummary) string {
	status := pr.Status
	if pr.IsDraft {
		status = "draft"
	}
	parts := []string{status}
	if pr.Approvals > 0 {
		parts = append(parts, prGoodStyle.Render(fmt.Sprintf("✓ approved ×%d", pr.Approvals)))
	} else {
		parts = append(parts, "not approved")
	}
	if pr.Rejections > 0 {
		parts = append(parts, prBadStyle.Render(fmt.Sprintf("✗ rejected ×%d", pr.Rejections)))
	}
	parts = append(parts, fmt.Sprintf("💬 %d", pr.CommentCount))
	return strings.Join(parts, prDimStyle.Render(" · "))
}

func pipelineRunLine(run azuredevops.PipelineRun) string {
	outcome := run.Status
	style := prDimStyle
	if run.Status == pipelineStatusDone {
		outcome = run.Result
		style = prBadStyle
		if run.Result == pipelineResultOK {
			style = prGoodStyle
		}
	}
	return fmt.Sprintf("▶ %s %s %s", run.Name, run.Number, style.Render(outcome))
}
