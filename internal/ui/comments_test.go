package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
	"cmdban/internal/task"
)

func testComments(count int) []azuredevops.Comment {
	comments := make([]azuredevops.Comment, count)
	for i := range comments {
		comments[i] = azuredevops.Comment{
			Text:        strings.Repeat("word ", 30),
			CreatedBy:   azuredevops.IdentityRef{DisplayName: "Ada"},
			CreatedDate: "2026-11-01T10:00:00Z",
		}
	}
	return comments
}

func withLoadedComments(m Model, comments []azuredevops.Comment) Model {
	got, _ := m.Update(commentsLoadedMsg{workItemID: m.commentsWorkItemID, comments: comments})
	return got.(Model)
}

func TestCommentsKeyOpensViewAndReturnsToPreviousMode(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.mode = ModeView

	m = pressKey(t, m, "c")
	if m.mode != ModeComments {
		t.Fatalf("mode = %v, want ModeComments", m.mode)
	}
	if m.commentsWorkItemID != m.selectedTask().ADOItemID || !m.commentsLoading {
		t.Fatalf("comments state = (id %d, loading %v), want loading for the selected task", m.commentsWorkItemID, m.commentsLoading)
	}

	m = pressKey(t, m, "q")
	if m.mode != ModeView {
		t.Fatalf("mode = %v after q, want ModeView", m.mode)
	}
}

func TestCommentsKeyOnLocalBoardShowsMessage(t *testing.T) {
	m := NewModel(&config.Config{CurrentBoard: "local", Boards: []config.Board{{Name: "local"}}})
	m.board.AddTask(&task.Task{ID: "1", Title: "local", Status: task.StatusToday})

	m = pressKey(t, m, "c")

	if m.mode == ModeComments || m.message == "" {
		t.Fatalf("mode = %v, message = %q; want to stay put with an explanation", m.mode, m.message)
	}
}

func TestCommentsLoadedForOtherWorkItemIsIgnored(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "c")

	got, _ := m.Update(commentsLoadedMsg{workItemID: m.commentsWorkItemID + 1, comments: testComments(1)})
	m = got.(Model)

	if !m.commentsLoading || len(m.comments) != 0 {
		t.Fatalf("stale comments applied: loading %v, %d comments", m.commentsLoading, len(m.comments))
	}
}

func TestCommentsViewScrollIsClamped(t *testing.T) {
	m := withLoadedComments(pressKey(t, adoTestModel(adoTestConfig()), "c"), testComments(20))

	m = pressKey(t, m, "G")
	maxScroll := m.maxCommentsScroll(m.width, m.bodyHeight())
	if maxScroll == 0 || m.commentsScroll != maxScroll {
		t.Fatalf("commentsScroll = %d after G, want max %d (> 0)", m.commentsScroll, maxScroll)
	}
	m = pressKey(t, m, "j")
	if m.commentsScroll != maxScroll {
		t.Fatalf("commentsScroll = %d after j at bottom, want clamped to %d", m.commentsScroll, maxScroll)
	}
}

func TestTabInTaskViewTogglesCommentsPane(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.mode = ModeView

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = withLoadedComments(got.(Model), testComments(2))
	if !m.viewCommentsPane {
		t.Fatal("viewCommentsPane = false after tab, want true")
	}
	if view := m.View(); !strings.Contains(view, "Comments (2)") || lipgloss.Height(view) != testHeight {
		t.Fatalf("task view should show the comments pane and fill the terminal:\n%s", view)
	}

	got, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got.(Model).viewCommentsPane {
		t.Fatal("viewCommentsPane = true after second tab, want false")
	}
}

func TestLeavingTaskViewClosesCommentsPane(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.mode = ModeView
	m.viewCommentsPane = true

	m = pressKey(t, m, "q")

	if m.viewCommentsPane {
		t.Fatal("viewCommentsPane = true after leaving the task view, want false")
	}
}

func TestCommentsPaneScrollKeysAreClamped(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.mode = ModeView
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = withLoadedComments(got.(Model), testComments(20))

	m = pressKey(t, m, "K")
	if m.commentsScroll != 0 {
		t.Fatalf("commentsScroll = %d after K at top, want 0", m.commentsScroll)
	}
	m = pressKey(t, m, "J")
	if m.commentsScroll != 1 {
		t.Fatalf("commentsScroll = %d after J, want 1", m.commentsScroll)
	}
}

func TestCommentLinesWrapToWidth(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.comments = testComments(2)
	const width = 40

	for _, line := range m.commentLines(width) {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

func TestTopBarShowsSelectedCardCommentCount(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.selectedTask().ADOCommentCount = 5

	if bar := m.renderTopBar(); !strings.Contains(bar, "Comments: 5") {
		t.Fatalf("topbar should show the selected card's comment count:\n%s", bar)
	}
}

func TestLocalTaskViewFillsTerminalHeight(t *testing.T) {
	path := t.TempDir() + "/task.md"
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 200)), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	m := NewModel(&config.Config{CurrentBoard: "local", Boards: []config.Board{{Name: "local"}}})
	m.width, m.height = testWidth, testHeight
	m.board.AddTask(&task.Task{ID: "1", Title: "local", Status: task.StatusToday, FilePath: path})
	m.mode = ModeView

	if got := lipgloss.Height(m.View()); got != testHeight {
		t.Fatalf("View() height = %d, want %d", got, testHeight)
	}
}

func writeCommentFile(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/comment.md"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestAddKeyInCommentsViewOpensEditor(t *testing.T) {
	m := withLoadedComments(pressKey(t, adoTestModel(adoTestConfig()), "c"), nil)

	_, cmd := m.Update(keyMsgFor("a"))

	if cmd == nil {
		t.Fatal("a in comments view returned no command, want the editor to open")
	}
}

func TestCommentEditorFinishedWithEmptyFileDiscards(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	path := writeCommentFile(t, "  \n\n")

	got, cmd := m.Update(commentEditorFinishedMsg{tempPath: path, workItemID: 1})

	if cmd != nil {
		t.Fatal("empty comment returned a command, want nothing posted")
	}
	if got.(Model).message == "" {
		t.Fatal("message = empty, want an explanation")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temp file still exists (err = %v), want it removed", err)
	}
}

func TestCommentEditorFinishedWithContentPosts(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	path := writeCommentFile(t, "**Looks good**\n")

	_, cmd := m.Update(commentEditorFinishedMsg{tempPath: path, workItemID: 1})

	if cmd == nil {
		t.Fatal("comment with content returned no command, want it posted")
	}
}

func TestCommentEditorErrorPostsNothing(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	path := writeCommentFile(t, "text")

	got, cmd := m.Update(commentEditorFinishedMsg{tempPath: path, workItemID: 1, execErr: errBoom})

	if cmd != nil || got.(Model).message == "" {
		t.Fatalf("editor error: cmd = %v, message = %q; want no post and an explanation", cmd, got.(Model).message)
	}
}

func TestCommentAddedReloadsOpenComments(t *testing.T) {
	m := withLoadedComments(pressKey(t, adoTestModel(adoTestConfig()), "c"), testComments(1))

	got, cmd := m.Update(commentAddedMsg{workItemID: m.commentsWorkItemID})
	m = got.(Model)

	if cmd == nil || !m.commentsLoading {
		t.Fatalf("cmd = %v, loading = %v; want comments reloading", cmd, m.commentsLoading)
	}
	m = withLoadedComments(m, testComments(2))
	if got := m.findTaskByADOItemID(m.commentsWorkItemID).ADOCommentCount; got != 2 {
		t.Fatalf("ADOCommentCount = %d, want 2 after reload", got)
	}
}

func TestCommentAddedForOtherWorkItemBumpsItsCount(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.commentsWorkItemID = 1

	got, cmd := m.Update(commentAddedMsg{workItemID: 2})
	m = got.(Model)

	if cmd != nil {
		t.Fatal("comment on a closed work item returned a command, want none")
	}
	if got := m.findTaskByADOItemID(2).ADOCommentCount; got != 1 {
		t.Fatalf("ADOCommentCount = %d, want 1", got)
	}
}
