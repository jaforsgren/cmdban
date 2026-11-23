package azuredevops

import (
	"fmt"
	"regexp"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

const (
	rawCheckboxChecked   = "[x]"
	rawCheckboxUnchecked = "[ ]"
)

var rawCheckboxMarkerRegex = regexp.MustCompile(`\[(?: |x|X|&nbsp;|&#160;|\x{00A0})\]`)

// SaveCheckboxChanges writes pending toggles to the work item. It patches the
// fields' raw values instead of converting edited markdown back, because
// markdownToHTML would drop what it doesn't model, such as images and tables.
// The revision test makes ADO reject the write if the item changed after the
// read.
func SaveCheckboxChanges(adoCfg *config.AzureDevOpsConfig, pat string, workItemID int, changes []task.CheckboxChange) error {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	item, err := client.FetchWorkItemBody(workItemID)
	if err != nil {
		return fmt.Errorf("reading work item %d: %w", workItemID, err)
	}
	description, reproSteps, err := applyCheckboxChanges(item.Fields.Description, item.Fields.ReproSteps, changes)
	if err != nil {
		return fmt.Errorf("work item %d: %w", workItemID, err)
	}
	ops := workItemBodyPatch(item, description, reproSteps)
	if len(ops) == 0 {
		return nil
	}
	if err := client.PatchWorkItem(workItemID, append([]PatchOperation{revisionTest(item.Rev)}, ops...)); err != nil {
		return fmt.Errorf("saving checkboxes on work item %d: %w", workItemID, err)
	}
	return nil
}

func revisionTest(rev int) PatchOperation {
	return PatchOperation{Op: "test", Path: "/rev", Value: rev}
}

func workItemBodyPatch(item WorkItem, description, reproSteps string) []PatchOperation {
	var ops []PatchOperation
	if description != item.Fields.Description {
		ops = append(ops, PatchOperation{Op: "add", Path: "/fields/" + fieldDescription, Value: description})
	}
	if reproSteps != item.Fields.ReproSteps {
		ops = append(ops, PatchOperation{Op: "add", Path: "/fields/" + fieldReproSteps, Value: reproSteps})
	}
	return ops
}

// applyCheckboxChanges applies on-screen checkbox numbers, which run through
// the description and on into the repro steps, to the raw field values.
func applyCheckboxChanges(description, reproSteps string, changes []task.CheckboxChange) (string, string, error) {
	descriptionBoxes := task.ParseCheckboxes(htmlToMarkdown(description))
	reproBoxes := task.ParseCheckboxes(htmlToMarkdown(reproSteps))
	boxes := append(descriptionBoxes, reproBoxes...)

	for _, change := range changes {
		if change.Number < 1 || change.Number > len(boxes) || boxes[change.Number-1].Label != change.Label {
			return "", "", fmt.Errorf("checkbox %d %q changed in Azure DevOps; discard and redo it", change.Number, change.Label)
		}
		var err error
		if change.Number <= len(descriptionBoxes) {
			description, err = setRawCheckbox(description, change.Number, change.Checked)
		} else {
			reproSteps, err = setRawCheckbox(reproSteps, change.Number-len(descriptionBoxes), change.Checked)
		}
		if err != nil {
			return "", "", err
		}
	}
	return description, reproSteps, nil
}

// setRawCheckbox flips one marker in a raw HTML or markdown field. Which
// marker is the numbered checkbox is decided by converting each candidate and
// comparing to the expected markdown, so the read-side conversion stays the
// only parser and a marker inside an attribute or code block is never hit.
func setRawCheckbox(raw string, number int, checked bool) (string, error) {
	current := htmlToMarkdown(raw)
	want, ok := task.SetCheckbox(current, number, checked)
	if !ok {
		return "", fmt.Errorf("no checkbox %d in field", number)
	}
	if want == current {
		return raw, nil
	}
	marker := rawCheckboxUnchecked
	if checked {
		marker = rawCheckboxChecked
	}
	for _, loc := range rawCheckboxMarkerRegex.FindAllStringIndex(raw, -1) {
		candidate := raw[:loc[0]] + marker + raw[loc[1]:]
		if htmlToMarkdown(candidate) == want {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not locate checkbox %d in the stored field", number)
}
