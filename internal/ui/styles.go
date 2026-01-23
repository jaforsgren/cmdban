package ui

import "github.com/charmbracelet/lipgloss"

var (
	HighlightColor = lipgloss.Color("#c3e7e8")
	SubtleColor    = lipgloss.Color("#626262")
	WarningColor   = lipgloss.Color("#ff4400")
	SuccessColor   = lipgloss.Color("#38b555")
	BorderColor    = lipgloss.Color("#3C3C3C")
	ActiveColor    = lipgloss.Color("#db6a39")
	InactiveColor  = lipgloss.Color("#7a7a7a")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(HighlightColor).
			MarginBottom(1)

	LaneHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	ActiveLaneHeaderStyle = LaneHeaderStyle.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ActiveColor)

	InactiveLaneHeaderStyle = LaneHeaderStyle.
				Foreground(SubtleColor)

	TaskStyle = lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(0)

	SelectedTaskStyle = TaskStyle.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ActiveColor)

	DoneTaskStyle = TaskStyle.
			Foreground(SubtleColor)

	SelectedDoneTaskStyle = TaskStyle.
				Foreground(lipgloss.Color("#AAAAAA")).
				Background(ActiveColor)

	LaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	ActiveLaneStyle = LaneStyle.
			BorderForeground(ActiveColor)

	HelpStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			MarginTop(1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			Padding(0, 1)

	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ActiveColor).
			Padding(0, 1)

	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ActiveColor).
			Padding(1, 2)
)

func GetLaneWidth(termWidth, numLanes int) int {
	padding := 4
	totalPadding := padding * numLanes
	return (termWidth - totalPadding) / numLanes
}
