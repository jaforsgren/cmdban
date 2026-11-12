package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

func stubBrowser(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	original := openInBrowser
	openInBrowser = func(url string) error {
		opened = append(opened, url)
		return nil
	}
	t.Cleanup(func() { openInBrowser = original })
	return &opened
}

func TestCtrlOOnADOBoardOpensBoardPage(t *testing.T) {
	opened := stubBrowser(t)
	cfg := adoTestConfig()
	cfg.Boards[1].AzureDevOps.AssignedToMe = true
	m := adoTestModel(cfg)

	m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})

	want := "https://dev.azure.com/org/proj/_boards/board/t/team/Stories?System.AssignedTo=%40me"
	if len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("opened %v, want [%s]", *opened, want)
	}
}

func TestCtrlOOnLocalBoardOpensTaskURL(t *testing.T) {
	opened := stubBrowser(t)
	m := NewModel(&config.Config{})
	m.board.AddTask(&task.Task{ID: "1", Title: "local", Status: m.board.Lanes[0], Description: "see https://example.com/doc"})

	m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})

	if len(*opened) != 1 || (*opened)[0] != "https://example.com/doc" {
		t.Fatalf("opened %v, want the task's URL", *opened)
	}
}

func TestCtrlOInCardOpensSelectedADOCard(t *testing.T) {
	for _, mode := range []Mode{ModeFocus, ModeView} {
		opened := stubBrowser(t)
		m := adoTestModel(adoTestConfig())
		m.mode = mode

		got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})

		want := "https://dev.azure.com/org/proj/_workitems/edit/1"
		if len(*opened) != 1 || (*opened)[0] != want {
			t.Fatalf("mode %v: opened %v, want [%s]", mode, *opened, want)
		}
		if got.(Model).message != "Opened "+want {
			t.Fatalf("mode %v: message = %q", mode, got.(Model).message)
		}
	}
}
