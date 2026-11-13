package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"cmdban/internal/task"
)

func TestSelectedCardKeepsHighlightBehindInlineSegments(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	activeBackground := lipgloss.NewStyle().Background(ActiveColor).Render(" ")
	activeBackgroundSGR := sgrParams(activeBackground)
	selectedMetaForegroundSGR := sgrParams(lipgloss.NewStyle().Foreground(SelectedMetaColor).Render(" "))

	m := adoTestModel(adoTestConfig())
	card := &task.Task{Title: "card", ADOItemID: 42, Tags: []string{"sejfa"}, CheckboxDone: 1, CheckboxTotal: 2}
	rendered := m.renderTask(card, true, 60, "")

	for _, segment := range []string{"#42", "@sejfa", "1/2"} {
		idx := strings.Index(rendered, segment)
		if idx < 0 {
			t.Fatalf("segment %q missing from %q", segment, rendered)
		}
		sgrStart := strings.LastIndex(rendered[:idx], "\x1b[")
		sgr := rendered[max(sgrStart, 0):idx]
		if sgrStart < 0 || !strings.Contains(sgr, activeBackgroundSGR) {
			t.Fatalf("segment %q rendered without selection background: %q", segment, rendered)
		}
		if !strings.Contains(sgr, selectedMetaForegroundSGR) {
			t.Fatalf("segment %q rendered without selected metadata colour: %q", segment, rendered)
		}
	}
}

func sgrParams(rendered string) string {
	return strings.TrimPrefix(rendered[:strings.Index(rendered, "m")], "\x1b[")
}
