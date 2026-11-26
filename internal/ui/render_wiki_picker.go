package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"cmdban/internal/azuredevops"
)

const (
	// The results and prompt take this share of the width, numerator over
	// denominator; the preview gets the rest.
	wikiPickerListShareNum = 1
	wikiPickerListShareDen = 2
	wikiPickerBorderCols   = 2
	wikiPickerBorderRows   = 2
	// wikiPickerTitleChromeCols are the corner, rule and spaces around a
	// box title.
	wikiPickerTitleChromeCols = 4
	wikiPickerPromptRows      = 1
	wikiPickerCursor          = "▶ "
	wikiPickerNoCursor        = "  "
	wikiPickerLineNoWidth     = 4
	// wikiPickerHitAnchor puts a grep hit this fraction of the preview's
	// height from its top, as numerator over denominator.
	wikiPickerHitAnchorNum = 1
	wikiPickerHitAnchorDen = 3
	wikiPickerNoResults    = "no matches"
	// wikiGrepTitleMax keeps a grep hit's page title short enough for the
	// matching text to stay visible beside it.
	wikiGrepTitleMax      = 24
	wikiPickerGrepHint    = "type to grep every page"
	wikiPickerNoFavorites = "no favorites yet — press f on a page to add it"
)

var (
	wikiPickerBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(BorderColor)
	wikiPickerTitle    = lipgloss.NewStyle().Bold(true).Foreground(ActiveColor)
	wikiPickerMatch    = lipgloss.NewStyle().Bold(true).Foreground(HighlightColor)
	wikiPickerDim      = lipgloss.NewStyle().Foreground(SubtleColor)
	wikiPickerHitLine  = lipgloss.NewStyle().Background(BorderColor)
)

// renderWikiPicker lays the picker out like telescope: the prompt over the
// results on the left, the selected page's text on the right.
func (m Model) renderWikiPicker() string {
	height := m.bodyHeight()
	listWidth := m.width * wikiPickerListShareNum / wikiPickerListShareDen
	previewWidth := m.width - listWidth

	prompt := m.renderPickerBox(listWidth, wikiPickerPromptRows, m.wikiPickerTitle()+"  "+m.pickerCount(), ansi.Truncate(m.textInput.View(), listWidth-wikiPickerBorderCols, ""), ActiveColor)
	resultsRows := max(height-lipgloss.Height(prompt)-wikiPickerBorderRows, 1)
	results := m.renderPickerBox(listWidth, resultsRows, "Results", m.renderPickerResults(listWidth-wikiPickerBorderCols, resultsRows), BorderColor)
	preview := m.renderPickerBox(previewWidth, height-wikiPickerBorderRows, m.pickerPreviewTitle(), m.renderPickerPreview(previewWidth-wikiPickerBorderCols, height-wikiPickerBorderRows), BorderColor)

	return lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Left, prompt, results), preview)
}

// renderPickerBox draws a bordered box of the given outer width and inner
// height with title set into its top border.
func (m Model) renderPickerBox(width, rows int, title, content string, border lipgloss.Color) string {
	innerWidth := max(width-wikiPickerBorderCols, 1)
	lines := strings.Split(content, "\n")
	lines = lines[:min(len(lines), rows)]
	box := wikiPickerBoxStyle.BorderForeground(border).Width(innerWidth).Height(rows).Render(strings.Join(lines, "\n"))

	boxLines := strings.Split(box, "\n")
	label := " " + ansi.Truncate(title, max(innerWidth-wikiPickerTitleChromeCols, 0), "…") + " "
	top := lipgloss.NewStyle().Foreground(border).Render("╭─") + wikiPickerTitle.Render(label)
	fill := max(width-lipgloss.Width(top)-1, 0)
	boxLines[0] = top + lipgloss.NewStyle().Foreground(border).Render(strings.Repeat("─", fill)+"╮")
	return strings.Join(boxLines, "\n")
}

func (m Model) pickerCount() string {
	if m.wiki.picker.kind != wikiPickerGrep {
		return fmt.Sprintf("%d / %d", len(m.wiki.picker.results), len(m.wikiPickerCandidates()))
	}
	return fmt.Sprintf("%d", len(m.wiki.picker.results))
}

func (m Model) renderPickerResults(width, rows int) string {
	results := m.wiki.picker.results
	if len(results) == 0 {
		hint := wikiPickerNoResults
		switch {
		case m.wiki.picker.kind == wikiPickerGrep && m.textInput.Value() == "":
			hint = wikiPickerGrepHint
		case m.wiki.picker.kind == wikiPickerFavorites && len(m.wikiPickerCandidates()) == 0:
			hint = wikiPickerNoFavorites
		}
		return wikiPickerDim.Italic(true).Render(hint)
	}

	start := max(min(m.wiki.picker.selected-rows/2, len(results)-rows), 0)
	end := min(start+rows, len(results))
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderPickerResult(results[i], i == m.wiki.picker.selected, width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderPickerResult(result wikiPickerResult, selected bool, width int) string {
	base := lipgloss.NewStyle()
	cursor := wikiPickerNoCursor
	if selected {
		base = SelectedTaskStyle.Padding(0)
		cursor = wikiPickerCursor
	}
	match := wikiPickerMatch.Background(base.GetBackground())
	dim := wikiPickerDim.Background(base.GetBackground())
	if selected {
		dim = dim.Foreground(SelectedMetaColor)
	}

	var line string
	if m.wiki.picker.kind == wikiPickerGrep {
		title := ansi.Truncate(azuredevops.WikiPageTitle(result.key.path), wikiGrepTitleMax, "…")
		line = cursor + dim.Render(fmt.Sprintf("%s:%d: ", title, result.line+1)) + highlightMatches(strings.TrimSpace(result.text), trimmedMatches(result.text, result.matches), base, match)
	} else {
		line = cursor + highlightMatches(result.label, result.matches, base, match)
	}
	return base.Width(width).Render(ansi.Truncate(line, width, "…"))
}

// trimmedMatches shifts match offsets past the leading whitespace that the
// result line drops.
func trimmedMatches(text string, matches []int) []int {
	shift := len(text) - len(strings.TrimLeft(text, " "))
	shifted := make([]int, 0, len(matches))
	for _, index := range matches {
		if index >= shift {
			shifted = append(shifted, index-shift)
		}
	}
	return shifted
}

// highlightMatches renders text with the runes at the byte offsets in
// matches in the match style.
func highlightMatches(text string, matches []int, base, match lipgloss.Style) string {
	var b strings.Builder
	for i, r := range text {
		if slices.Contains(matches, i) {
			b.WriteString(match.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

func (m Model) pickerPreviewTitle() string {
	if result, ok := m.selectedPickerResult(); ok {
		return result.label
	}
	return "Preview"
}

// renderPickerPreview shows the selected page's markdown with line numbers;
// a grep hit is highlighted and kept in view.
func (m Model) renderPickerPreview(width, rows int) string {
	result, ok := m.selectedPickerResult()
	if !ok {
		return ""
	}
	lines := strings.Split(m.wikiPageBody(result.key), "\n")

	start := m.wiki.picker.previewScroll
	if m.wiki.picker.kind == wikiPickerGrep {
		start += max(result.line-rows*wikiPickerHitAnchorNum/wikiPickerHitAnchorDen, 0)
	}
	start = max(min(start, len(lines)-1), 0)
	end := min(start+rows, len(lines))

	textWidth := max(width-wikiPickerLineNoWidth-1, 1)
	rendered := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		number := wikiPickerDim.Render(fmt.Sprintf("%*d ", wikiPickerLineNoWidth, i+1))
		text := ansi.Truncate(strings.ReplaceAll(lines[i], "\t", " "), textWidth, "…")
		if m.wiki.picker.kind == wikiPickerGrep && i == result.line {
			text = wikiPickerHitLine.Width(textWidth).Render(text)
		}
		rendered = append(rendered, number+text)
	}
	return strings.Join(rendered, "\n")
}
