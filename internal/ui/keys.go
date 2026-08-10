package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Top        key.Binding
	Bottom     key.Binding
	HalfUp     key.Binding
	HalfDown   key.Binding
	MoveLeft   key.Binding
	MoveRight  key.Binding
	OrderUp    key.Binding
	OrderDown  key.Binding
	New        key.Binding
	Edit       key.Binding
	Delete     key.Binding
	ToggleDone key.Binding
	Mark       key.Binding
	Enter      key.Binding
	Escape     key.Binding
	Settings   key.Binding
	Help       key.Binding
	Quit       key.Binding
	Search     key.Binding
	Filter     key.Binding
	Refresh    key.Binding
	OpenEditor key.Binding
	OpenURL    key.Binding
	Comments   key.Binding
	Tag        key.Binding
	Boards         key.Binding
	CollapseColumn key.Binding
}

var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/←", "left lane"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/→", "right lane"),
	),
	Top: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("gg", "top"),
	),
	Bottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "bottom"),
	),
	HalfUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "half page up"),
	),
	HalfDown: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "half page down"),
	),
	MoveLeft: key.NewBinding(
		key.WithKeys("H", "shift+left"),
		key.WithHelp("H/shift+←", "move task left"),
	),
	MoveRight: key.NewBinding(
		key.WithKeys("L", "shift+right"),
		key.WithHelp("L/shift+→", "move task right"),
	),
	OrderUp: key.NewBinding(
		key.WithKeys("ctrl+k", "shift+up"),
		key.WithHelp("ctrl+k/shift+↑", "move task up"),
	),
	OrderDown: key.NewBinding(
		key.WithKeys("ctrl+j", "shift+down"),
		key.WithHelp("ctrl+j/shift+↓", "move task down"),
	),
	New: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "new task"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e", "i"),
		key.WithHelp("e/i", "edit task"),
	),
	Delete: key.NewBinding(
		key.WithKeys("ctrl+d", "x"),
		key.WithHelp("ctrl+d/x", "delete task"),
	),
	ToggleDone: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "toggle done"),
	),
	Mark: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "mark task"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Settings: key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "command mode"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter by tag"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r", "ctrl+r"),
		key.WithHelp("r", "refresh"),
	),
	OpenEditor: key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp("ctrl+g", "open in editor"),
	),
	OpenURL: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "open url in browser"),
	),
	Comments: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "view comments"),
	),
	Tag: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "add tag"),
	),
	Boards: key.NewBinding(
		key.WithKeys("ctrl+b"),
		key.WithHelp("ctrl+b", "board switcher"),
	),
	CollapseColumn: key.NewBinding(
		key.WithKeys("."),
		key.WithHelp(".", "minimize column"),
	),
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.New, k.ToggleDone, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Top, k.Bottom, k.HalfUp, k.HalfDown},
		{k.MoveLeft, k.MoveRight, k.OrderUp, k.OrderDown},
		{k.New, k.Edit, k.ToggleDone, k.Delete},
		{k.Mark, k.Tag, k.Search, k.Filter, k.Refresh},
		{k.Settings, k.Help, k.Quit, k.Boards, k.CollapseColumn},
		{k.OpenEditor, k.OpenURL, k.Comments},
	}
}
