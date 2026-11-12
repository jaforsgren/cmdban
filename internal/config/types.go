package config

import (
	"maps"
	"slices"
	"strings"
)

type BoardType string

const (
	BoardTypeLocal       BoardType = "local"
	BoardTypeAzureDevOps BoardType = "azuredevops"
)

type AzureDevOpsConfig struct {
	Org                 string            `yaml:"org"`
	Project             string            `yaml:"project"`
	Team                string            `yaml:"team"`
	Iteration           string            `yaml:"iteration,omitempty"`
	BacklogLevel        string            `yaml:"backlog_level,omitempty"`
	AssignedToMe        bool              `yaml:"assigned_to_me,omitempty"`
	PAT                 string            `yaml:"pat"`
	ColumnMap           map[string]string `yaml:"column_map,omitempty"`
	DefaultWorkItemType string            `yaml:"default_work_item_type,omitempty"`
	// BoardLevel names the team Kanban board (e.g. "Stories") whose columns
	// the lanes mirror. It is independent of BacklogLevel, which scopes the
	// fetched work items.
	BoardLevel       string               `yaml:"board_level,omitempty"`
	BoardColumnField string               `yaml:"board_column_field,omitempty"`
	BoardColumns     []BoardColumnMapping `yaml:"board_columns,omitempty"`
	LaneMode         LaneMode             `yaml:"lane_mode,omitempty"`
}

// LaneMode picks what a board-column ADO board's lanes represent. The zero
// value means Kanban columns.
type LaneMode string

const (
	LaneModeColumns LaneMode = "columns"
	LaneModeStates  LaneMode = "states"
)

// BoardColumnMapping ties a cmdban lane to an ADO Kanban board column and the
// work item state each type must be in to sit in that column.
type BoardColumnMapping struct {
	Lane   string            `yaml:"lane"`
	Column string            `yaml:"column"`
	States map[string]string `yaml:"states"`
}

// UsesBoardColumns reports whether lanes map to Kanban board columns rather
// than raw work item states (the legacy ColumnMap behaviour).
func (c *AzureDevOpsConfig) UsesBoardColumns() bool {
	return c.BoardColumnField != "" && len(c.BoardColumns) > 0
}

// ShowsStateLanes reports whether a board-column board is currently toggled
// to one lane per work item state instead of one per Kanban column.
func (c *AzureDevOpsConfig) ShowsStateLanes() bool {
	return c.UsesBoardColumns() && c.LaneMode == LaneModeStates
}

// ToggleLaneMode flips between Kanban column lanes and state lanes.
func (c *AzureDevOpsConfig) ToggleLaneMode() {
	if c.ShowsStateLanes() {
		c.LaneMode = LaneModeColumns
		return
	}
	c.LaneMode = LaneModeStates
}

// StateLanes returns one lane per distinct work item state, in board column
// order, so lanes read left to right the way the Kanban board does.
func (c *AzureDevOpsConfig) StateLanes() []StateLane {
	var lanes []StateLane
	seen := map[string]bool{}
	for _, bc := range c.BoardColumns {
		for _, workItemType := range slices.Sorted(maps.Keys(bc.States)) {
			state := bc.States[workItemType]
			lane := LaneSlug(state)
			if seen[lane] {
				continue
			}
			seen[lane] = true
			lanes = append(lanes, StateLane{Lane: lane, State: state})
		}
	}
	return lanes
}

// StateLane ties a lane to the work item state it shows when a board is in
// state lane mode.
type StateLane struct {
	Lane  string
	State string
}

// LaneSlug turns an ADO column or state name into a lane name.
func LaneSlug(s string) string {
	var b strings.Builder
	lastWasDash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastWasDash = false
		} else if b.Len() > 0 && !lastWasDash {
			b.WriteRune('-')
			lastWasDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

type Column struct {
	Name   string `yaml:"name"`
	Hidden bool   `yaml:"hidden"`
}

type Board struct {
	Name        string             `yaml:"name"`
	Directory   string             `yaml:"directory,omitempty"`
	Type        BoardType          `yaml:"type,omitempty"`
	AzureDevOps *AzureDevOpsConfig `yaml:"azuredevops,omitempty"`
	Columns     []Column           `yaml:"columns"`
}

// LaneColumns returns the board's lanes. A board-column ADO board toggled to
// state lanes derives them from its state mappings; a state lane is hidden
// only when every Kanban column showing that state is hidden.
func (b *Board) LaneColumns() []Column {
	if b.AzureDevOps == nil || !b.AzureDevOps.ShowsStateLanes() {
		return b.Columns
	}
	hiddenByLane := make(map[string]bool, len(b.Columns))
	for _, col := range b.Columns {
		hiddenByLane[col.Name] = col.Hidden
	}
	allHidden := map[string]bool{}
	for _, bc := range b.AzureDevOps.BoardColumns {
		for _, state := range bc.States {
			lane := LaneSlug(state)
			hidden, seen := allHidden[lane]
			allHidden[lane] = hiddenByLane[bc.Lane] && (hidden || !seen)
		}
	}
	stateLanes := b.AzureDevOps.StateLanes()
	columns := make([]Column, len(stateLanes))
	for i, sl := range stateLanes {
		columns[i] = Column{Name: sl.Lane, Hidden: allHidden[sl.Lane]}
	}
	return columns
}

type Config struct {
	CurrentBoard string   `yaml:"current_board"`
	Boards       []Board  `yaml:"boards"`
	IgnoreFiles  []string `yaml:"ignore_files,omitempty"`
	RecentBoards []string `yaml:"recent_boards,omitempty"`
	configPath   string
}

const MaxRecentBoards = 4

// TouchRecentBoard moves name to the front of the most-recently-used list.
func (c *Config) TouchRecentBoard(name string) {
	recent := []string{name}
	for _, existing := range c.RecentBoards {
		if existing != name {
			recent = append(recent, existing)
		}
	}
	c.RecentBoards = recent[:min(len(recent), MaxRecentBoards)]
}

// RecentBoardNames returns the recently used boards that still exist, most
// recent first, always including the current board.
func (c *Config) RecentBoardNames() []string {
	var names []string
	if current := c.ActiveBoard(); current != nil && !slices.Contains(c.RecentBoards, current.Name) {
		names = append(names, current.Name)
	}
	for _, name := range c.RecentBoards {
		if c.BoardByName(name) != nil {
			names = append(names, name)
		}
	}
	return names[:min(len(names), MaxRecentBoards)]
}

// NextRecentBoard returns the board after the current one in the recent
// list, wrapping around, or "" when there is nothing to cycle to.
func (c *Config) NextRecentBoard() string {
	names := c.RecentBoardNames()
	if len(names) < 2 {
		return ""
	}
	current := slices.Index(names, c.ActiveBoard().Name)
	return names[(current+1)%len(names)]
}

func (c *Config) BoardByName(name string) *Board {
	for i := range c.Boards {
		if c.Boards[i].Name == name {
			return &c.Boards[i]
		}
	}
	return nil
}

func (c *Config) ActiveBoard() *Board {
	if board := c.BoardByName(c.CurrentBoard); board != nil {
		return board
	}
	if len(c.Boards) > 0 {
		return &c.Boards[0]
	}
	return nil
}

func (c *Config) TaskDirectory() string {
	if board := c.ActiveBoard(); board != nil {
		return board.Directory
	}
	return DefaultTaskDir
}

func (c *Config) VisibleColumns() []string {
	board := c.ActiveBoard()
	if board == nil {
		return DefaultColumns
	}

	var visible []string
	for _, col := range board.LaneColumns() {
		if !col.Hidden {
			visible = append(visible, col.Name)
		}
	}
	return visible
}

func (c *Config) AllColumns() []string {
	board := c.ActiveBoard()
	if board == nil {
		return DefaultColumns
	}

	laneColumns := board.LaneColumns()
	columns := make([]string, len(laneColumns))
	for i, col := range laneColumns {
		columns[i] = col.Name
	}
	return columns
}

func (c *Config) IsColumnHidden(column string) bool {
	board := c.ActiveBoard()
	if board == nil {
		return false
	}

	for _, col := range board.LaneColumns() {
		if col.Name == column {
			return col.Hidden
		}
	}
	return false
}

func (c *Config) AddBoard(name, directory string) {
	columns := make([]Column, len(DefaultColumns))
	for i, col := range DefaultColumns {
		columns[i] = Column{Name: col}
	}
	c.Boards = append(c.Boards, Board{
		Name:      name,
		Directory: directory,
		Columns:   columns,
	})
}

func (c *Config) ConfigPath() string {
	return c.configPath
}

func (c *Config) HiddenColumns() []string {
	board := c.ActiveBoard()
	if board == nil {
		return nil
	}

	var hidden []string
	for _, col := range board.LaneColumns() {
		if col.Hidden {
			hidden = append(hidden, col.Name)
		}
	}
	return hidden
}
