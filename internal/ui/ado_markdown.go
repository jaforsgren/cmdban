package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/azuredevops"
	"cmdban/internal/task"
)

const markdownNone = "_None._"

type adoMarkdownReadyMsg struct {
	path string
}

type adoMarkdownClosedMsg struct {
	path    string
	execErr error
}

// openADOMarkdown fetches the selected work item's comments, writes the item
// as a markdown document and opens it in $EDITOR for reading; the file is a
// temporary copy, removed when the editor exits.
func (m Model) openADOMarkdown() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || t.ADOItemID == 0 {
		m.message = "Markdown view is only available for Azure DevOps work items"
		return m, nil
	}
	url, _ := m.taskURL(t)
	adoCfg := m.config.ActiveBoard().AzureDevOps
	snapshot := *t
	m.message = fmt.Sprintf("Preparing #%d as markdown...", t.ADOItemID)
	return m, func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		comments, err := azuredevops.FetchComments(adoCfg, token, snapshot.ADOItemID)
		if err != nil {
			return errMsg{err}
		}
		path, err := writeADOMarkdownFile(&snapshot, url, comments)
		if err != nil {
			return errMsg{fmt.Errorf("writing markdown for #%d: %w", snapshot.ADOItemID, err)}
		}
		return adoMarkdownReadyMsg{path: path}
	}
}

func (m Model) handleADOMarkdownReady(msg adoMarkdownReadyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	return m, tea.ExecProcess(editorCommand(msg.path), func(err error) tea.Msg {
		return adoMarkdownClosedMsg{path: msg.path, execErr: err}
	})
}

func (m Model) handleADOMarkdownClosed(msg adoMarkdownClosedMsg) (tea.Model, tea.Cmd) {
	os.Remove(msg.path)
	if msg.execErr != nil {
		m.message = "Editor error: " + msg.execErr.Error()
		m.logs.Error(m.message)
	}
	return m, nil
}

func writeADOMarkdownFile(t *task.Task, url string, comments []azuredevops.Comment) (string, error) {
	f, err := os.CreateTemp("", fmt.Sprintf("cmdban-ado-view-%d-*.md", t.ADOItemID))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(adoTaskMarkdown(t, url, comments)); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// adoTaskMarkdown renders a work item as a readable markdown document:
// title, a metadata table, description, repro steps and comments.
func adoTaskMarkdown(t *task.Task, url string, comments []azuredevops.Comment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", t.ADOItemID, t.Title)

	b.WriteString("| Field | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Type | %s |\n", markdownCell(t.ADOWorkItemType))
	fmt.Fprintf(&b, "| Lane | %s |\n", markdownCell(string(t.Status)))
	fmt.Fprintf(&b, "| Tags | %s |\n", markdownCell(strings.Join(markdownTags(t), ", ")))
	fmt.Fprintf(&b, "| Comments | %d |\n", len(comments))
	fmt.Fprintf(&b, "| Link | %s |\n\n", markdownCell(url))

	b.WriteString("## Description\n\n")
	b.WriteString(markdownBody(t.Description))

	if strings.TrimSpace(t.ReproSteps) != "" {
		b.WriteString("## Repro Steps\n\n")
		b.WriteString(markdownBody(t.ReproSteps))
	}

	fmt.Fprintf(&b, "## Comments (%d)\n\n", len(comments))
	if len(comments) == 0 {
		b.WriteString(markdownNone + "\n")
	}
	for i, c := range comments {
		if i > 0 {
			b.WriteString("---\n\n")
		}
		fmt.Fprintf(&b, "### %s — %s\n\n", c.CreatedBy.DisplayName, formatCommentDate(c.CreatedDate))
		b.WriteString(markdownBody(c.Text))
	}
	return b.String()
}

// markdownTags drops the work item type, which sync stores as the first tag
// but the table already shows as its own row.
func markdownTags(t *task.Task) []string {
	var tags []string
	for _, tag := range t.Tags {
		if tag != t.ADOWorkItemType {
			tags = append(tags, "`"+tag+"`")
		}
	}
	return tags
}

func markdownCell(value string) string {
	if value == "" {
		return "—"
	}
	return strings.ReplaceAll(value, "|", `\|`)
}

func markdownBody(text string) string {
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		return trimmed + "\n\n"
	}
	return markdownNone + "\n\n"
}
