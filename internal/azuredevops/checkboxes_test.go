package azuredevops

import (
	"strings"
	"testing"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

func TestApplyCheckboxChangesKeepsUnmodelledHTML(t *testing.T) {
	description := `<div><img src="a.png"></div><table><tr><td>[ ] cell</td></tr></table><ul><li>[ ] one</li><li>[&nbsp;] two</li></ul>`

	got, _, err := applyCheckboxChanges(description, "", []task.CheckboxChange{{Number: 2, Label: "two", Checked: true}})
	if err != nil {
		t.Fatal(err)
	}

	want := strings.Replace(description, "[&nbsp;] two", "[x] two", 1)
	if got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestApplyCheckboxChangesTargetsDuplicateLabelByPosition(t *testing.T) {
	description := "<ul><li>[ ] same</li><li>[ ] same</li></ul>"

	got, _, err := applyCheckboxChanges(description, "", []task.CheckboxChange{{Number: 2, Label: "same", Checked: true}})
	if err != nil {
		t.Fatal(err)
	}

	if want := "<ul><li>[ ] same</li><li>[x] same</li></ul>"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestApplyCheckboxChangesNumbersThroughReproSteps(t *testing.T) {
	description, repro, err := applyCheckboxChanges("<ul><li>[ ] one</li></ul>", "- [x] two", []task.CheckboxChange{{Number: 2, Label: "two", Checked: false}})
	if err != nil {
		t.Fatal(err)
	}

	if description != "<ul><li>[ ] one</li></ul>" || repro != "- [ ] two" {
		t.Fatalf("description %q, repro %q", description, repro)
	}
}

func TestApplyCheckboxChangesRefusesChangedLabel(t *testing.T) {
	_, _, err := applyCheckboxChanges("<ul><li>[ ] renamed</li></ul>", "", []task.CheckboxChange{{Number: 1, Label: "one", Checked: true}})

	if err == nil {
		t.Fatal("error = nil, want a conflict")
	}
}

func TestWorkItemToTaskCarriesAuthorAndCheckboxCounts(t *testing.T) {
	item := WorkItem{Fields: WorkItemFields{
		ID:          1,
		Description: "<ul><li>[x] one</li><li>[ ] two</li></ul>",
		CreatedBy:   &IdentityRef{DisplayName: "Ada Author"},
	}}

	got := workItemToTask(item, testCheckboxBoardConfig())

	if got.ADOAuthor != "Ada Author" {
		t.Errorf("ADOAuthor = %q", got.ADOAuthor)
	}
	if got.CheckboxDone != 1 || got.CheckboxTotal != 2 {
		t.Errorf("counts = %d/%d, want 1/2", got.CheckboxDone, got.CheckboxTotal)
	}
}

func testCheckboxBoardConfig() *config.AzureDevOpsConfig {
	return &config.AzureDevOpsConfig{ColumnMap: map[string]string{"backlog": "New"}}
}

func TestSavePatchGuardsRevisionAndSendsOnlyChangedField(t *testing.T) {
	item := WorkItem{Rev: 9, Fields: WorkItemFields{Description: "- [ ] one", ReproSteps: "- [ ] two"}}

	ops := append([]PatchOperation{revisionTest(item.Rev)}, workItemBodyPatch(item, "- [x] one", item.Fields.ReproSteps)...)

	if len(ops) != 2 || ops[0].Op != "test" || ops[0].Path != "/rev" || ops[0].Value != 9 {
		t.Fatalf("ops = %+v, want a revision test then one field", ops)
	}
	if ops[1].Path != "/fields/"+fieldDescription || ops[1].Value != "- [x] one" {
		t.Fatalf("field op = %+v", ops[1])
	}
}
