package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	maxVisibleTagSuggestions = 8
	noTagSuggestionSelected  = -1
)

// tagAutocomplete lists the board's tags matching what has been typed so
// far. An empty input matches every tag, so the list doubles as a picker.
type tagAutocomplete struct {
	candidates []string
	matches    []string
	selected   int
}

func newTagAutocomplete(candidates []string) tagAutocomplete {
	sorted := slices.Clone(candidates)
	slices.SortFunc(sorted, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	a := tagAutocomplete{candidates: sorted}
	a.filter("")
	return a
}

// filter keeps the tags containing input, ignoring case, with tags that
// start with input listed first. The selection is cleared, since its index
// no longer points at the same tag.
func (a *tagAutocomplete) filter(input string) {
	needle := strings.ToLower(strings.TrimSpace(input))
	var prefixed, contained []string
	for _, tag := range a.candidates {
		lower := strings.ToLower(tag)
		switch {
		case strings.HasPrefix(lower, needle):
			prefixed = append(prefixed, tag)
		case strings.Contains(lower, needle):
			contained = append(contained, tag)
		}
	}
	a.matches = append(prefixed, contained...)
	a.selected = noTagSuggestionSelected
}

// moveSelection steps through the matches, wrapping at both ends; stepping
// down from no selection lands on the first match.
func (a *tagAutocomplete) moveSelection(delta int) {
	if len(a.matches) == 0 {
		return
	}
	if a.selected == noTagSuggestionSelected {
		if delta > 0 {
			a.selected = 0
		} else {
			a.selected = len(a.matches) - 1
		}
		return
	}
	a.selected = (a.selected + delta + len(a.matches)) % len(a.matches)
}

func (a tagAutocomplete) selection() (string, bool) {
	if a.selected == noTagSuggestionSelected {
		return "", false
	}
	return a.matches[a.selected], true
}

// completion is the tag tab completes to: the selected match, or else the
// best match.
func (a tagAutocomplete) completion() (string, bool) {
	if tag, ok := a.selection(); ok {
		return tag, true
	}
	if len(a.matches) == 0 {
		return "", false
	}
	return a.matches[0], true
}

// visibleWindow returns the slice of matches to draw, scrolled so the
// selection stays in view, and the index of its first entry.
func (a tagAutocomplete) visibleWindow() ([]string, int) {
	if len(a.matches) <= maxVisibleTagSuggestions {
		return a.matches, 0
	}
	start := 0
	if a.selected >= maxVisibleTagSuggestions {
		start = a.selected - maxVisibleTagSuggestions + 1
	}
	return a.matches[start : start+maxVisibleTagSuggestions], start
}

func (a tagAutocomplete) render() string {
	if len(a.matches) == 0 {
		return lipgloss.NewStyle().Foreground(SubtleColor).Render("  no matching tags — enter adds a new one")
	}
	visible, start := a.visibleWindow()
	selectedStyle := lipgloss.NewStyle().Foreground(HighlightColor).Bold(true)
	subtleStyle := lipgloss.NewStyle().Foreground(SubtleColor)
	lines := make([]string, 0, len(visible)+1)
	for i, tag := range visible {
		if start+i == a.selected {
			lines = append(lines, selectedStyle.Render("› "+tag))
		} else {
			lines = append(lines, subtleStyle.Render("  "+tag))
		}
	}
	if hidden := len(a.matches) - len(visible); hidden > 0 {
		lines = append(lines, subtleStyle.Render(fmt.Sprintf("  … %d more", hidden)))
	}
	return strings.Join(lines, "\n")
}
