package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/task"
)

// preferredWorkItemTypeOrder keeps the type filter's number keys stable
// across syncs; types outside this list follow alphabetically.
var preferredWorkItemTypeOrder = []string{"User Story", "Bug", "Task", "Feature", "Epic"}

const typeFilterAll = 0

// workItemTypes lists the distinct ADO work item types on the board. Index i
// is selected with number key i+1.
func (m Model) workItemTypes() []string {
	seen := map[string]bool{}
	var types []string
	for _, lane := range m.board.Lanes {
		for _, t := range m.board.Tasks[lane] {
			if t.ADOWorkItemType != "" && !seen[t.ADOWorkItemType] {
				seen[t.ADOWorkItemType] = true
				types = append(types, t.ADOWorkItemType)
			}
		}
	}
	slices.SortFunc(types, compareWorkItemTypes)
	return types
}

func compareWorkItemTypes(a, b string) int {
	return cmp.Or(cmp.Compare(workItemTypeRank(a), workItemTypeRank(b)), cmp.Compare(a, b))
}

func workItemTypeRank(workItemType string) int {
	if i := slices.Index(preferredWorkItemTypeOrder, workItemType); i >= 0 {
		return i
	}
	return len(preferredWorkItemTypeOrder)
}

func (m Model) workItemTypeCount(workItemType string) int {
	count := 0
	for _, lane := range m.board.Lanes {
		for _, t := range m.board.Tasks[lane] {
			if t.ADOWorkItemType == workItemType {
				count++
			}
		}
	}
	return count
}

func (m Model) selectTypeFilter(digit string) (tea.Model, tea.Cmd) {
	if !m.config.IsADOBoard() {
		m.message = "Type filter only applies to Azure DevOps boards"
		return m, nil
	}
	index, err := strconv.Atoi(digit)
	if err != nil {
		m.message = fmt.Sprintf("Invalid type filter key %q", digit)
		return m, nil
	}
	if index == typeFilterAll {
		m.typeFilter = ""
		m.clampSelection()
		return m, nil
	}
	types := m.workItemTypes()
	if index > len(types) {
		m.message = fmt.Sprintf("No work item type on key %d", index)
		return m, nil
	}
	m.typeFilter = types[index-1]
	m.clampSelection()
	return m, nil
}

func (m Model) matchesTypeFilter(t *task.Task) bool {
	return m.typeFilter == "" || t.ADOWorkItemType == m.typeFilter
}

func (m Model) hasActiveFilter() bool {
	return m.searchFilter != "" || m.typeFilter != ""
}

// toggleLaneMode switches a board-column ADO board between Kanban column
// lanes and work item state lanes, then re-syncs so tasks land in the new
// lanes.
func (m Model) toggleLaneMode() (tea.Model, tea.Cmd) {
	if !m.config.IsADOBoard() || !m.config.ActiveBoard().AzureDevOps.UsesBoardColumns() {
		m.message = "Lane toggle needs an ADO board with board columns — run :adocolumns"
		return m, nil
	}
	adoCfg := m.config.ActiveBoard().AzureDevOps
	adoCfg.ToggleLaneMode()
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	m.board = task.NewBoardWithColumns(m.config.AllColumns(), m.config.HiddenColumns())
	m.activeLane = 0
	m.activeTask = 0
	m.minimizedLanes = make(map[task.Status]bool)
	m.message = "Lanes: " + laneModeLabel(adoCfg.ShowsStateLanes()) + " — syncing..."
	return m, m.loadTasks
}

func laneModeLabel(stateLanes bool) string {
	if stateLanes {
		return "states"
	}
	return "board columns"
}
