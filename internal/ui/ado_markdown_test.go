package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
	"cmdban/internal/task"
)

func markdownTestTask() *task.Task {
	return &task.Task{
		ADOItemID:       2240,
		Title:           "Login fails",
		Status:          "review",
		ADOWorkItemType: "Bug",
		Tags:            []string{"Bug", "Johan Forsgren", "backend|auth"},
		Description:     "Users can't log in.\n\n- step one",
		ReproSteps:      "1. Open app",
	}
}

func TestADOTaskMarkdownIncludesAllSections(t *testing.T) {
	comments := []azuredevops.Comment{
		{Text: "**Looks** fixed", CreatedBy: azuredevops.IdentityRef{DisplayName: "Ada"}, CreatedDate: "2026-11-01T10:00:00Z"},
		{Text: "Verified", CreatedBy: azuredevops.IdentityRef{DisplayName: "Bob"}, CreatedDate: "2026-11-02T10:00:00Z"},
	}

	md := adoTaskMarkdown(markdownTestTask(), "https://dev.azure.com/org/proj/_workitems/edit/2240", comments)

	for _, want := range []string{
		"# #2240 Login fails",
		"| Type | Bug |",
		"| Lane | review |",
		"`Johan Forsgren`",
		`backend\|auth`,
		"| Link | https://dev.azure.com/org/proj/_workitems/edit/2240 |",
		"## Description\n\nUsers can't log in.\n\n- step one",
		"## Repro Steps\n\n1. Open app",
		"## Comments (2)",
		"### Ada — ",
		"**Looks** fixed",
		"---",
		"### Bob — ",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "`Bug`") {
		t.Error("work item type should not be repeated as a tag")
	}
}

func TestADOTaskMarkdownMarksEmptySections(t *testing.T) {
	tk := markdownTestTask()
	tk.Description, tk.ReproSteps = "", ""

	md := adoTaskMarkdown(tk, "", nil)

	if !strings.Contains(md, "## Description\n\n"+markdownNone) || !strings.Contains(md, "## Comments (0)\n\n"+markdownNone) {
		t.Fatalf("empty sections should say %q:\n%s", markdownNone, md)
	}
	if strings.Contains(md, "## Repro Steps") {
		t.Fatal("empty repro steps should be left out")
	}
	if !strings.Contains(md, "| Link | — |") {
		t.Fatal("missing link should show a dash")
	}
}

func TestMarkdownKeyStartsPreparingForADOCard(t *testing.T) {
	for _, mode := range []Mode{ModeNormal, ModeView, ModeFocus} {
		m := adoTestModel(adoTestConfig())
		m.mode = mode

		got, cmd := m.Update(keyMsgFor("M"))

		if cmd == nil || !strings.Contains(got.(Model).message, "#1") {
			t.Fatalf("mode %v: cmd = %v, message = %q; want the markdown being prepared", mode, cmd, got.(Model).message)
		}
	}
}

func TestMarkdownKeyOnLocalTaskShowsMessage(t *testing.T) {
	m := NewModel(&config.Config{})
	m.board.AddTask(&task.Task{ID: "1", Title: "local", Status: m.board.Lanes[0]})

	got, cmd := m.Update(keyMsgFor("M"))

	if cmd != nil || got.(Model).message == "" {
		t.Fatalf("cmd = %v, message = %q; want nothing started and an explanation", cmd, got.(Model).message)
	}
}

func TestMarkdownFileOpensInEditorAndIsRemovedAfter(t *testing.T) {
	path, err := writeADOMarkdownFile(markdownTestTask(), "https://example/2240", nil)
	if err != nil {
		t.Fatalf("writeADOMarkdownFile() error = %v", err)
	}
	m := adoTestModel(adoTestConfig())

	if _, cmd := m.Update(adoMarkdownReadyMsg{path: path}); cmd == nil {
		t.Fatal("ready markdown returned no command, want the editor opened")
	}

	got, _ := m.Update(adoMarkdownClosedMsg{path: path, execErr: errBoom})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("markdown file still exists (err = %v), want it removed", err)
	}
	if !strings.Contains(got.(Model).message, "boom") {
		t.Fatalf("message = %q, want the editor error", got.(Model).message)
	}
}

func TestADODetailBodyRendersMarkdownWrappedToWidth(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	tk := markdownTestTask()
	tk.Description = "Intro **bold** " + strings.Repeat("word ", 40) + "\n\n- first\n- second"
	const width = 40

	lines := m.adoDetailBodyLines(tk, width)
	joined := strings.Join(lines, "\n")

	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
	for _, want := range []string{"• first", "• second", "Repro Steps", "Open app"} {
		if !strings.Contains(joined, want) {
			t.Errorf("detail body missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "**") || strings.Contains(joined, "## ") {
		t.Fatalf("markdown syntax should be rendered, not shown raw:\n%s", joined)
	}
}
