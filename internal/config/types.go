package config

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
	ColumnMap           map[string]string `yaml:"column_map"`
	DefaultWorkItemType string            `yaml:"default_work_item_type,omitempty"`
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

type Config struct {
	CurrentBoard string  `yaml:"current_board"`
	Boards       []Board `yaml:"boards"`
	configPath   string
}

func (c *Config) ActiveBoard() *Board {
	for i := range c.Boards {
		if c.Boards[i].Name == c.CurrentBoard {
			return &c.Boards[i]
		}
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
	for _, col := range board.Columns {
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

	columns := make([]string, len(board.Columns))
	for i, col := range board.Columns {
		columns[i] = col.Name
	}
	return columns
}

func (c *Config) IsColumnHidden(column string) bool {
	board := c.ActiveBoard()
	if board == nil {
		return false
	}

	for _, col := range board.Columns {
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
	for _, col := range board.Columns {
		if col.Hidden {
			hidden = append(hidden, col.Name)
		}
	}
	return hidden
}
