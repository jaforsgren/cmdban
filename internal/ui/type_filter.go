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

// Number keys for the type filter: 0 is the default stories-and-bugs view,
// 1–8 pick a single type and 9 shows every type.
const (
	typeFilterKeyDefault  = 0
	typeFilterKeyAllTypes = 9
	maxTypeFilterTypes    = typeFilterKeyAllTypes - 1
)

type typeFilterKind int

const (
	typeFilterDefault typeFilterKind = iota
	typeFilterAllTypes
	typeFilterSingle
)

// typeFilter is the active work item type selection on an ADO board.
type typeFilter struct {
	kind         typeFilterKind
	workItemType string
}

// defaultViewTypes are the types the default (0) view shows.
var defaultViewTypes = []string{"User Story", "Bug"}

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
	types := m.workItemTypes()
	switch {
	case index == typeFilterKeyDefault:
		m.typeFilter = typeFilter{kind: typeFilterDefault}
	case index == typeFilterKeyAllTypes:
		m.typeFilter = typeFilter{kind: typeFilterAllTypes}
	case index > len(types):
		m.message = fmt.Sprintf("No work item type on key %d", index)
		return m, nil
	default:
		m.typeFilter = typeFilter{kind: typeFilterSingle, workItemType: types[index-1]}
	}
	m.clampSelection()
	return m, nil
}

// typeFilterMatcher returns the predicate for the active type filter, which
// applies only on ADO boards. The default view falls back to every type when
// the board carries no stories or bugs, so boards such as an Epics backlog
// never look empty.
func (m Model) typeFilterMatcher() func(*task.Task) bool {
	matchAll := func(*task.Task) bool { return true }
	if !m.config.IsADOBoard() {
		return matchAll
	}
	switch m.typeFilter.kind {
	case typeFilterSingle:
		return func(t *task.Task) bool { return t.ADOWorkItemType == m.typeFilter.workItemType }
	case typeFilterDefault:
		if !m.hasDefaultViewTypes() {
			return matchAll
		}
		return func(t *task.Task) bool { return slices.Contains(defaultViewTypes, t.ADOWorkItemType) }
	default:
		return matchAll
	}
}

func (m Model) hasDefaultViewTypes() bool {
	return slices.ContainsFunc(m.workItemTypes(), func(workItemType string) bool {
		return slices.Contains(defaultViewTypes, workItemType)
	})
}

// hasActiveFilter reports a filter the user applied, which esc clears; the
// default type view doesn't count.
func (m Model) hasActiveFilter() bool {
	return m.searchFilter != "" || m.typeFilter.kind != typeFilterDefault
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
