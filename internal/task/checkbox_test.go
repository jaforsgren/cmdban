package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCheckboxesFindsListVariantsAndSkipsCodeAndLinks(t *testing.T) {
	markdown := "- [ ] one\n  * [x] two\n1. [X] three\n- [x](https://example.com)\n```\n- [ ] sample\n```\nplain [ ] text"

	boxes := ParseCheckboxes(markdown)

	want := []Checkbox{{"one", false}, {"two", true}, {"three", true}}
	if len(boxes) != len(want) {
		t.Fatalf("ParseCheckboxes() = %v, want %v", boxes, want)
	}
	for i := range want {
		if boxes[i] != want[i] {
			t.Errorf("box %d = %v, want %v", i, boxes[i], want[i])
		}
	}
}

func TestSetCheckboxChangesOnlyTheNumberedBox(t *testing.T) {
	got, ok := SetCheckbox("intro\n- [ ] one\n- [ ] two\r\nend", 2, true)

	if want := "intro\n- [ ] one\n- [x] two\r\nend"; !ok || got != want {
		t.Fatalf("SetCheckbox() = %q, %v, want %q", got, ok, want)
	}
}

func TestSetCheckboxRejectsMissingNumber(t *testing.T) {
	for _, number := range []int{0, 2} {
		if _, ok := SetCheckbox("- [ ] one", number, true); ok {
			t.Errorf("SetCheckbox(%d) ok = true, want false", number)
		}
	}
}

func TestNumberCheckboxesStartsAtFirst(t *testing.T) {
	got := NumberCheckboxes("- [ ] one\ntext\n- [x] two", 3)

	if want := "- (3) [ ] one\ntext\n- (4) [x] two"; got != want {
		t.Fatalf("NumberCheckboxes() = %q, want %q", got, want)
	}
}

func TestToggleCheckboxInFileKeepsTheRestOfTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.md")
	if err := os.WriteFile(path, []byte("# Title\n\n- [ ] one\n\n---\n@status today\n"), taskFileMode); err != nil {
		t.Fatal(err)
	}

	if err := ToggleCheckboxInFile(path, 1); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if want := "# Title\n\n- [x] one\n\n---\n@status today\n"; string(got) != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestToggleCheckboxInFileRejectsMissingNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.md")
	if err := os.WriteFile(path, []byte("# Title\n\n- [ ] one\n"), taskFileMode); err != nil {
		t.Fatal(err)
	}

	if err := ToggleCheckboxInFile(path, 2); err == nil {
		t.Fatal("ToggleCheckboxInFile(2) error = nil, want an error")
	}
}

func TestTaskSetCheckboxNumbersThroughReproSteps(t *testing.T) {
	card := &Task{Description: "- [ ] one", ReproSteps: "- [ ] two"}

	if !card.SetCheckbox(2, true) {
		t.Fatal("SetCheckbox(2) = false")
	}

	if card.Description != "- [ ] one" || card.ReproSteps != "- [x] two" {
		t.Fatalf("description %q, repro %q", card.Description, card.ReproSteps)
	}
	if card.CheckboxDone != 1 || card.CheckboxTotal != 2 {
		t.Fatalf("counts = %d/%d, want 1/2", card.CheckboxDone, card.CheckboxTotal)
	}
}
