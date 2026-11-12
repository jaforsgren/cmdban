package ui

import (
	"slices"
	"strings"
	"testing"

	"cmdban/internal/commentread"
	"cmdban/internal/task"
)

// unreadTestModel syncs the board once so every card's comments count as
// read, then gives "Story one" a new comment.
func unreadTestModel(t *testing.T) Model {
	t.Helper()
	m := adoTestModel(adoTestConfig())
	m.commentReads = commentread.New(t.TempDir() + "/reads.yaml")
	got, _ := m.Update(tasksLoadedMsg{tasks: allTasks(m)})
	m = got.(Model)
	m.findTaskByADOItemID(2).ADOCommentCount++
	return m
}

func allTasks(m Model) []*task.Task {
	var tasks []*task.Task
	for _, lane := range m.board.Lanes {
		tasks = append(tasks, m.board.Tasks[lane]...)
	}
	return tasks
}

func TestCardWithNewCommentShowsUnreadMarker(t *testing.T) {
	m := unreadTestModel(t)

	if !strings.Contains(m.renderTask(m.findTaskByADOItemID(2), false, testWidth, ""), unreadCommentsMarker) {
		t.Fatal("card with a new comment has no unread marker")
	}
	if strings.Contains(m.renderTask(m.findTaskByADOItemID(1), false, testWidth, ""), unreadCommentsMarker) {
		t.Fatal("card without new comments shows the unread marker")
	}
}

func TestUnreadCommentsKeyShowsOnlyUnreadCards(t *testing.T) {
	m := pressKey(t, unreadTestModel(t), "8")

	if got := shownTitles(m); !slices.Equal(got, []string{"Story one"}) {
		t.Fatalf("key 8 shown = %v, want only the card with unread comments", got)
	}
	if !m.hasActiveFilter() {
		t.Fatal("hasActiveFilter() = false for the unread view, want true so esc clears it")
	}
}

func TestLoadingCommentsMarksThemRead(t *testing.T) {
	m := unreadTestModel(t)
	m.commentsWorkItemID = 2

	m = withLoadedComments(m, testComments(m.findTaskByADOItemID(2).ADOCommentCount))

	if m.hasUnreadComments(m.findTaskByADOItemID(2)) {
		t.Fatal("card still unread after its comments were loaded")
	}
}

func TestOwnCommentOnOtherWorkItemIsMarkedRead(t *testing.T) {
	m := unreadTestModel(t)
	m.commentsWorkItemID = 2

	got, _ := m.Update(commentAddedMsg{workItemID: 1})
	m = got.(Model)

	if m.hasUnreadComments(m.findTaskByADOItemID(1)) {
		t.Fatal("the user's own comment shows as unread")
	}
}

func TestFiltersSectionListsUnreadCommentsKey(t *testing.T) {
	m := unreadTestModel(t)

	section := strings.Join(m.buildFilterSection(), "\n")
	if !strings.Contains(section, "<8>") || !strings.Contains(section, "unread (1)") {
		t.Fatalf("FILTERS section = %q, want the <8> unread (1) row", section)
	}
}

func TestUnreadCommentsInHiddenColumnsAreNotCounted(t *testing.T) {
	cfg := adoTestConfig()
	cfg.Boards[1].Columns[1].Hidden = true
	m := NewModel(cfg)
	m.width, m.height = testWidth, testHeight
	m.commentReads = commentread.New(t.TempDir() + "/reads.yaml")
	hidden := &task.Task{ID: "2", Title: "Hidden story", Status: "review", ADOWorkItemType: "User Story", ADOItemID: 2}
	got, _ := m.Update(tasksLoadedMsg{tasks: []*task.Task{hidden}})
	m = got.(Model)
	hidden.ADOCommentCount++

	if got := m.unreadCommentsTaskCount(); got != 0 {
		t.Fatalf("unreadCommentsTaskCount() = %d, want 0 for a card in a hidden column", got)
	}
	if got := shownTitles(pressKey(t, m, "8")); len(got) != 0 {
		t.Fatalf("key 8 shown = %v, want nothing from hidden columns", got)
	}
}
