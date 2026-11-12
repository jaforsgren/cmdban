package ui

import "github.com/charmbracelet/lipgloss"

type typeGlyph struct {
	symbol string
	color  lipgloss.Color
}

// typeGlyphs are the small markers shown before card titles and in the
// topbar type legend; colours are muted so they don't compete with the
// selection highlight.
var (
	typeGlyphs = map[string]typeGlyph{
		"User Story": {"◆", lipgloss.Color("#6c9bd2")},
		"Bug":        {"●", lipgloss.Color("#c96a6a")},
		"Task":       {"▪", lipgloss.Color("#c9b46a")},
		"Feature":    {"★", lipgloss.Color("#a58bd0")},
		"Epic":       {"♛", lipgloss.Color("#d09a6a")},
	}
	fallbackTypeGlyph = typeGlyph{"•", SubtleColor}
)

func glyphForType(workItemType string) typeGlyph {
	if glyph, ok := typeGlyphs[workItemType]; ok {
		return glyph
	}
	return fallbackTypeGlyph
}

// typeGlyphCellWidth is the padding plus glyph column a card gives up for
// its type marker.
const typeGlyphCellWidth = 2

func renderTypeGlyph(workItemType string) string {
	glyph := glyphForType(workItemType)
	return lipgloss.NewStyle().Foreground(glyph.color).Render(glyph.symbol)
}

// renderTypeGlyphCell renders the card's type marker as its own column, on
// the card's background so a selected card stays highlighted edge to edge.
// It's a separate cell because embedding styled text in the title would
// reset the card's background after the glyph.
func renderTypeGlyphCell(workItemType string, background lipgloss.TerminalColor, height int) string {
	glyph := glyphForType(workItemType)
	return lipgloss.NewStyle().
		Foreground(glyph.color).
		Background(background).
		PaddingLeft(1).
		Width(typeGlyphCellWidth).
		Height(height).
		Render(glyph.symbol)
}
