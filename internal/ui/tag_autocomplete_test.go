package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

func TestTagAutocompleteEmptyInputMatchesEveryTagSorted(t *testing.T) {
	a := newTagAutocomplete([]string{"otel", "API", "backend"})

	want := []string{"API", "backend", "otel"}
	if !reflect.DeepEqual(a.matches, want) {
		t.Fatalf("matches = %v, want %v", a.matches, want)
	}
}

func TestTagAutocompleteFilterListsPrefixMatchesBeforeContains(t *testing.T) {
	a := newTagAutocomplete([]string{"sejfa", "api", "backend-api", "apidocs"})

	a.filter("API")

	want := []string{"api", "apidocs", "backend-api"}
	if !reflect.DeepEqual(a.matches, want) {
		t.Fatalf("matches = %v, want %v", a.matches, want)
	}
}

func TestTagAutocompleteFilterClearsSelection(t *testing.T) {
	a := newTagAutocomplete([]string{"a", "b"})
	a.moveSelection(1)

	a.filter("b")

	if _, ok := a.selection(); ok {
		t.Fatal("selection() ok = true after filter, want the selection cleared")
	}
}

func TestTagAutocompleteMoveSelectionWraps(t *testing.T) {
	a := newTagAutocomplete([]string{"a", "b", "c"})

	a.moveSelection(-1)
	if got, _ := a.selection(); got != "c" {
		t.Fatalf("up from no selection = %q, want last match c", got)
	}
	a.moveSelection(1)
	if got, _ := a.selection(); got != "a" {
		t.Fatalf("down from last = %q, want wrap to a", got)
	}
}

func TestTagAutocompleteMoveSelectionWithoutMatchesKeepsNoSelection(t *testing.T) {
	a := newTagAutocomplete(nil)

	a.moveSelection(1)

	if _, ok := a.selection(); ok {
		t.Fatal("selection() ok = true with no matches, want false")
	}
}

func TestTagAutocompleteCompletionFallsBackToBestMatch(t *testing.T) {
	a := newTagAutocomplete([]string{"backend", "api"})
	a.filter("b")

	if got, ok := a.completion(); !ok || got != "backend" {
		t.Fatalf("completion() = %q, %v, want backend, true", got, ok)
	}
}

func TestTagAutocompleteVisibleWindowFollowsSelection(t *testing.T) {
	var tags []string
	for _, c := range "abcdefghijkl" {
		tags = append(tags, string(c))
	}
	a := newTagAutocomplete(tags)
	for range maxVisibleTagSuggestions + 1 {
		a.moveSelection(1)
	}

	visible, start := a.visibleWindow()

	if len(visible) != maxVisibleTagSuggestions || start != 1 {
		t.Fatalf("window = %v starting at %d, want %d entries starting at 1", visible, start, maxVisibleTagSuggestions)
	}
	if !strings.Contains(a.render(), "more") {
		t.Fatal("render() does not mention the hidden matches")
	}
}

func adoTagTestModel() Model {
	cfg := &config.Config{
		CurrentBoard: "ado",
		Boards: []config.Board{{
			Name:        "ado",
			Type:        config.BoardTypeAzureDevOps,
			AzureDevOps: &config.AzureDevOpsConfig{Org: "org", Project: "proj"},
		}},
	}
	m := NewModel(cfg)
	m.board.AddTask(&task.Task{
		ID: "ado-1", Status: task.StatusToday, ADOItemID: 1,
		Tags: []string{"User Story", "Jane Doe", "api"}, ADOTags: []string{"api"},
	})
	m.board.AddTask(&task.Task{
		ID: "ado-2", Status: task.StatusToday, ADOItemID: 2,
		Tags: []string{"Bug", "otel", "API"}, ADOTags: []string{"otel", "API"},
	})
	return m
}

func TestTagCandidatesOnADOBoardOnlyOfferWorkItemTags(t *testing.T) {
	m := adoTagTestModel()
	selected := m.board.Tasks[task.StatusToday][0]

	got := m.tagCandidates(selected)

	if want := []string{"otel"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tagCandidates() = %v, want %v (no type, assignee, or tags the card already has)", got, want)
	}
}

func TestTagModeOnADOBoardListsTagsBeforeTyping(t *testing.T) {
	m := adoTagTestModel()

	got, _ := m.handleNormalMode(keyMsgFor("t"))
	updated := got.(Model)

	if updated.mode != ModeTag {
		t.Fatalf("mode = %v, want ModeTag on an ADO board", updated.mode)
	}
	if want := []string{"otel"}; !reflect.DeepEqual(updated.tagAutocomplete.matches, want) {
		t.Fatalf("matches = %v, want %v", updated.tagAutocomplete.matches, want)
	}
}

func TestTagModeEnterAddsSelectedSuggestion(t *testing.T) {
	m := NewModel(&config.Config{})
	m.board.AddTask(&task.Task{ID: "1", Status: task.StatusToday, Tags: []string{"backend"}})
	target := &task.Task{ID: "2", Status: task.StatusToday, FilePath: t.TempDir() + "/2.md"}
	m.board.AddTask(target)
	m.activeTask = 1

	got, _ := m.enterTagMode(target)
	got, _ = got.(Model).handleTagMode(tea.KeyMsg{Type: tea.KeyDown})
	got, cmd := got.(Model).handleTagMode(tea.KeyMsg{Type: tea.KeyEnter})

	if got.(Model).mode != ModeNormal {
		t.Fatalf("mode = %v, want ModeNormal after enter", got.(Model).mode)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a save command")
	}
	if want := []string{"backend"}; !reflect.DeepEqual(target.Tags, want) {
		t.Fatalf("Tags = %v, want %v", target.Tags, want)
	}
}

func TestAddTagToTaskRejectsTagDifferingOnlyInCase(t *testing.T) {
	m := adoTagTestModel()

	got, cmd := m.addTagToTask("API")

	if cmd != nil {
		t.Fatal("cmd = non-nil, want no ADO write for a tag the card already has")
	}
	if got.(Model).message != "Tag already exists" {
		t.Fatalf("message = %q, want the duplicate warning", got.(Model).message)
	}
}
