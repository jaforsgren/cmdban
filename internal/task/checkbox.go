package task

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"cmdban/internal/fsutil"
)

const (
	checkboxMarkChecked   = "x"
	checkboxMarkUnchecked = " "
	checkboxCodeFence     = "```"
	markdownLinkTarget    = "("
)

// checkboxLineRegex matches a task list item in a bullet or numbered list, at
// any nesting depth. "- [x](url)" also matches and is rejected by
// parseCheckboxLine, since RE2 has no lookahead and that line is a link.
var checkboxLineRegex = regexp.MustCompile(`^(\s*(?:[-*+]|\d+\.)\s+)\[([ xX])\](.*)$`)

type checkboxLine struct {
	index  int
	prefix string
	rest   string
	box    Checkbox
}

func parseCheckboxLine(index int, line string) (checkboxLine, bool) {
	parts := checkboxLineRegex.FindStringSubmatch(line)
	if parts == nil || strings.HasPrefix(parts[3], markdownLinkTarget) {
		return checkboxLine{}, false
	}
	return checkboxLine{
		index:  index,
		prefix: parts[1],
		rest:   parts[3],
		box:    Checkbox{Label: strings.TrimSpace(parts[3]), Checked: parts[2] != checkboxMarkUnchecked},
	}, true
}

// checkboxLines finds the checkboxes in markdown, skipping fenced code, where
// a "- [ ]" is sample text rather than a checkbox.
func checkboxLines(lines []string) []checkboxLine {
	var found []checkboxLine
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), checkboxCodeFence) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if parsed, ok := parseCheckboxLine(i, line); ok {
			found = append(found, parsed)
		}
	}
	return found
}

// ParseCheckboxes lists markdown's checkboxes in document order.
func ParseCheckboxes(markdown string) []Checkbox {
	parsed := checkboxLines(strings.Split(markdown, "\n"))
	boxes := make([]Checkbox, len(parsed))
	for i, line := range parsed {
		boxes[i] = line.box
	}
	return boxes
}

func CountCheckboxes(markdown string) (done, total int) {
	boxes := ParseCheckboxes(markdown)
	for _, box := range boxes {
		if box.Checked {
			done++
		}
	}
	return done, len(boxes)
}

// SetCheckbox sets the 1-based numbered checkbox, leaving every other byte of
// markdown as is. It reports false when there is no such checkbox.
func SetCheckbox(markdown string, number int, checked bool) (string, bool) {
	lines := strings.Split(markdown, "\n")
	parsed := checkboxLines(lines)
	if number < 1 || number > len(parsed) {
		return markdown, false
	}
	target := parsed[number-1]
	mark := checkboxMarkUnchecked
	if checked {
		mark = checkboxMarkChecked
	}
	lines[target.index] = target.prefix + "[" + mark + "]" + target.rest
	return strings.Join(lines, "\n"), true
}

// NumberCheckboxes prefixes each checkbox with the number that toggles it,
// starting at first. The number goes before the box because "[x](1)" would
// render as a link.
func NumberCheckboxes(markdown string, first int) string {
	lines := strings.Split(markdown, "\n")
	for offset, line := range checkboxLines(lines) {
		mark := checkboxMarkUnchecked
		if line.box.Checked {
			mark = checkboxMarkChecked
		}
		lines[line.index] = line.prefix + "(" + strconv.Itoa(first+offset) + ") [" + mark + "]" + line.rest
	}
	return strings.Join(lines, "\n")
}

// ToggleCheckboxInFile flips one checkbox in place rather than going through
// WriteMarkdownFile, so the rest of the file keeps its exact formatting.
func ToggleCheckboxInFile(path string, number int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading task file %s: %w", path, err)
	}
	content := string(data)
	boxes := ParseCheckboxes(content)
	if number < 1 || number > len(boxes) {
		return fmt.Errorf("task has no checkbox %d (it has %d)", number, len(boxes))
	}
	updated, _ := SetCheckbox(content, number, !boxes[number-1].Checked)
	if err := fsutil.WriteFileAtomic(path, []byte(updated), taskFileMode); err != nil {
		return fmt.Errorf("writing task file %s: %w", path, err)
	}
	return nil
}

// Checkboxes lists the task's checkboxes as numbered on screen: description
// first, then repro steps.
func (t *Task) Checkboxes() []Checkbox {
	return append(ParseCheckboxes(t.Description), ParseCheckboxes(t.ReproSteps)...)
}

// SetCheckbox sets a checkbox by its on-screen number and refreshes the
// task's counts. It reports false when there is no such checkbox.
func (t *Task) SetCheckbox(number int, checked bool) bool {
	_, descriptionTotal := CountCheckboxes(t.Description)
	var ok bool
	if number <= descriptionTotal {
		t.Description, ok = SetCheckbox(t.Description, number, checked)
	} else {
		t.ReproSteps, ok = SetCheckbox(t.ReproSteps, number-descriptionTotal, checked)
	}
	if ok {
		t.RefreshCheckboxCounts()
	}
	return ok
}

func (t *Task) RefreshCheckboxCounts() {
	descriptionDone, descriptionTotal := CountCheckboxes(t.Description)
	reproDone, reproTotal := CountCheckboxes(t.ReproSteps)
	t.CheckboxDone = descriptionDone + reproDone
	t.CheckboxTotal = descriptionTotal + reproTotal
}
