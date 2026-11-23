package checkboxdraft

import (
	"path/filepath"
	"slices"
	"testing"

	"cmdban/internal/task"
)

const testOrg = "org"

func TestToggleTwiceClearsTheDraft(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "drafts.yaml"))
	change := task.CheckboxChange{Number: 1, Label: "one", Checked: true}

	store.Toggle(testOrg, 7, change)
	store.Toggle(testOrg, 7, task.CheckboxChange{Number: 1, Label: "one", Checked: false})

	if got := store.Changes(testOrg, 7); len(got) != 0 {
		t.Fatalf("Changes() = %v, want none", got)
	}
}

func TestDraftsSurviveSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.yaml")
	store := New(path)
	change := task.CheckboxChange{Number: 2, Label: "two", Checked: true}
	store.Toggle(testOrg, 7, change)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := loaded.Changes(testOrg, 7); !slices.Equal(got, []task.CheckboxChange{change}) {
		t.Fatalf("Changes() = %v, want %v", got, change)
	}
}

func TestRemoveKeepsChangesMadeDuringSave(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "drafts.yaml"))
	saved := task.CheckboxChange{Number: 1, Label: "one", Checked: true}
	later := task.CheckboxChange{Number: 2, Label: "two", Checked: true}
	store.Toggle(testOrg, 7, saved)
	store.Toggle(testOrg, 7, later)

	store.Remove(testOrg, 7, []task.CheckboxChange{saved})

	if got := store.Changes(testOrg, 7); !slices.Equal(got, []task.CheckboxChange{later}) {
		t.Fatalf("Changes() = %v, want only %v", got, later)
	}
}
