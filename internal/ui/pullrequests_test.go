package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
	"cmdban/internal/task"
)

func testPRSummary() azuredevops.PullRequestSummary {
	return azuredevops.PullRequestSummary{
		ID: 3261, Title: "fix: login", Status: "completed", URL: "https://example/pr/3261",
		Description: "## Problem\n\nLogin was optional.", Approvals: 1, CommentCount: 2,
		Pipelines: []azuredevops.PipelineRun{{Name: "CI", Number: "0.10.6", Status: "completed", Result: "succeeded", URL: "https://example/build/1"}},
	}
}

func taskViewModel(t *testing.T) Model {
	t.Helper()
	m := adoTestModel(adoTestConfig())
	m.mode = ModeView
	return m
}

func TestTabLoadsLinkedPRsAndComments(t *testing.T) {
	got, cmd := taskViewModel(t).Update(tea.KeyMsg{Type: tea.KeyTab})
	m := got.(Model)

	if cmd == nil || !m.linkedPRsLoading || !m.commentsLoading {
		t.Fatalf("cmd = %v, prs loading %v, comments loading %v; want both loading", cmd, m.linkedPRsLoading, m.commentsLoading)
	}
	if m.linkedPRsWorkItemID != m.selectedTask().ADOItemID {
		t.Fatalf("linkedPRsWorkItemID = %d, want the selected task", m.linkedPRsWorkItemID)
	}
}

func TestLinkedPRsLoadedForOtherWorkItemIsIgnored(t *testing.T) {
	got, _ := taskViewModel(t).Update(tea.KeyMsg{Type: tea.KeyTab})
	m := got.(Model)

	got, _ = m.Update(linkedPRsLoadedMsg{workItemID: m.linkedPRsWorkItemID + 1, prs: []azuredevops.PullRequestSummary{testPRSummary()}})

	if m = got.(Model); !m.linkedPRsLoading || len(m.linkedPRs) != 0 {
		t.Fatalf("stale PRs applied: loading %v, %d PRs", m.linkedPRsLoading, len(m.linkedPRs))
	}
}

func TestSidePaneShowsPRSummaryAndFillsHeight(t *testing.T) {
	got, _ := taskViewModel(t).Update(tea.KeyMsg{Type: tea.KeyTab})
	m := got.(Model)
	got, _ = m.Update(linkedPRsLoadedMsg{workItemID: m.linkedPRsWorkItemID, prs: []azuredevops.PullRequestSummary{testPRSummary()}})
	m = got.(Model)

	view := m.View()
	for _, want := range []string{"#3261 fix: login", "approved ×1", "💬 2", "https://example/pr/3261", "Login was optional.", "CI 0.10.6", "succeeded", "Comments ("} {
		if !strings.Contains(view, want) {
			t.Errorf("side pane missing %q", want)
		}
	}
	if got := lipgloss.Height(view); got != testHeight {
		t.Fatalf("View() height = %d, want %d", got, testHeight)
	}
}

func TestPullRequestStatusLineShowsRejectionsAndDraft(t *testing.T) {
	line := pullRequestStatusLine(azuredevops.PullRequestSummary{Status: "active", IsDraft: true, Rejections: 1})

	for _, want := range []string{"draft", "not approved", "rejected ×1"} {
		if !strings.Contains(line, want) {
			t.Errorf("status line %q missing %q", line, want)
		}
	}
}

func TestLinkKeyOpensPickerForADOTask(t *testing.T) {
	m := pressKey(t, taskViewModel(t), "l")

	if m.mode != ModePRPicker || !m.prPicker.loading || m.prPicker.workItemID != 1 {
		t.Fatalf("mode = %v, picker = %+v; want a loading picker for #1", m.mode, m.prPicker)
	}
}

func TestLinkKeyOnLocalTaskShowsMessage(t *testing.T) {
	m := NewModel(&config.Config{CurrentBoard: "local", Boards: []config.Board{{Name: "local"}}})
	m.board.AddTask(&task.Task{ID: "1", Title: "local", Status: task.StatusToday})
	m.mode = ModeView

	m = pressKey(t, m, "l")

	if m.mode != ModeView || m.message == "" {
		t.Fatalf("mode = %v, message = %q; want to stay in the view with an explanation", m.mode, m.message)
	}
}

func pickerWithPRs(t *testing.T) Model {
	t.Helper()
	m := pressKey(t, taskViewModel(t), "l")
	got, _ := m.Update(myPRsLoadedMsg{workItemID: m.prPicker.workItemID, prs: []azuredevops.LinkablePullRequest{
		{Ref: azuredevops.PullRequestRef{ID: 30}, Title: "newest"},
		{Ref: azuredevops.PullRequestRef{ID: 20}, Title: "older"},
	}})
	return got.(Model)
}

func TestPRPickerNavigatesAndLinksSelection(t *testing.T) {
	m := pressKey(t, pickerWithPRs(t), "k")
	if m.prPicker.index != 1 {
		t.Fatalf("index = %d after k, want wrap to 1", m.prPicker.index)
	}

	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = got.(Model)

	if cmd == nil || m.mode != ModeView || !strings.Contains(m.message, "#20") {
		t.Fatalf("cmd = %v, mode = %v, message = %q; want PR #20 being linked", cmd, m.mode, m.message)
	}
}

func TestPRPickerEscapeReturnsToTaskView(t *testing.T) {
	m := pressKey(t, pickerWithPRs(t), "q")

	if m.mode != ModeView {
		t.Fatalf("mode = %v, want ModeView", m.mode)
	}
}

func TestPRPickerEnterWhileLoadingDoesNothing(t *testing.T) {
	m := pressKey(t, taskViewModel(t), "l")

	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil || got.(Model).mode != ModePRPicker {
		t.Fatal("enter while loading should be ignored")
	}
}

func TestPRLinkedReloadsOpenSidePane(t *testing.T) {
	got, _ := taskViewModel(t).Update(tea.KeyMsg{Type: tea.KeyTab})
	m := got.(Model)
	got, _ = m.Update(linkedPRsLoadedMsg{workItemID: m.linkedPRsWorkItemID})
	m = got.(Model)

	got, cmd := m.Update(prLinkedMsg{workItemID: m.linkedPRsWorkItemID, prID: 20})
	m = got.(Model)

	if cmd == nil || !m.linkedPRsLoading || !strings.Contains(m.message, "#20") {
		t.Fatalf("cmd = %v, loading = %v, message = %q; want PRs reloading", cmd, m.linkedPRsLoading, m.message)
	}
}
