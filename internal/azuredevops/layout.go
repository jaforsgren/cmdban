package azuredevops

import (
	"fmt"

	"cmdban/internal/config"
)

// ApplyBoardLayout rewrites a board's lanes to mirror the ADO Kanban board's
// columns. Lanes that already exist keep their hidden flag; new outgoing
// (done) columns start hidden, matching how the web board collapses them.
func ApplyBoardLayout(board *config.Board, detail KanbanBoardDetail) {
	wasHidden := make(map[string]bool, len(board.Columns))
	for _, col := range board.Columns {
		wasHidden[col.Name] = col.Hidden
	}

	lanes := uniqueLaneSlugs(detail.Columns)
	columns := make([]config.Column, len(detail.Columns))
	mappings := make([]config.BoardColumnMapping, len(detail.Columns))
	for i, bc := range detail.Columns {
		hidden, existed := wasHidden[lanes[i]]
		if !existed {
			hidden = bc.ColumnType == boardColumnTypeOutgoing
		}
		columns[i] = config.Column{Name: lanes[i], Hidden: hidden}
		mappings[i] = config.BoardColumnMapping{Lane: lanes[i], Column: bc.Name, States: bc.StateMappings}
	}

	board.Columns = columns
	board.AzureDevOps.BoardLevel = detail.Name
	board.AzureDevOps.BoardColumnField = detail.Fields.ColumnField.ReferenceName
	board.AzureDevOps.BoardColumns = mappings
	board.AzureDevOps.ColumnMap = nil
}

// uniqueLaneSlugs slugifies each column name, suffixing duplicates with an
// index so every lane name stays unique.
func uniqueLaneSlugs(boardColumns []BoardColumn) []string {
	slugCounts := map[string]int{}
	slugs := make([]string, len(boardColumns))
	for i, bc := range boardColumns {
		slugs[i] = config.LaneSlug(bc.Name)
		slugCounts[slugs[i]]++
	}

	seen := map[string]int{}
	for i, slug := range slugs {
		if slugCounts[slug] > 1 {
			seen[slug]++
			slugs[i] = fmt.Sprintf("%s-%d", slug, seen[slug])
		}
	}
	return slugs
}
