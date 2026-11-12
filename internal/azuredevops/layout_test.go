package azuredevops

import (
	"reflect"
	"testing"

	"cmdban/internal/config"
)

const testColumnField = "WEF_ABC_Kanban.Column"

func storiesBoardDetail() KanbanBoardDetail {
	return KanbanBoardDetail{
		Name: "Stories",
		Fields: BoardFields{
			ColumnField: FieldReference{ReferenceName: testColumnField},
		},
		Columns: []BoardColumn{
			{Name: "ToDo", ColumnType: "incoming", StateMappings: map[string]string{"User Story": "New", "Bug": "New"}},
			{Name: "In progress", ColumnType: "inProgress", StateMappings: map[string]string{"User Story": "Active", "Bug": "Active"}},
			{Name: "Review", ColumnType: "inProgress", StateMappings: map[string]string{"User Story": "Active", "Bug": "Active"}},
			{Name: "Waiting for build", ColumnType: "inProgress", StateMappings: map[string]string{"User Story": "Resolved", "Bug": "Resolved"}},
			{Name: "Closed", ColumnType: "outgoing", StateMappings: map[string]string{"User Story": "Closed", "Bug": "Closed"}},
		},
	}
}

func TestApplyBoardLayoutMirrorsBoardColumns(t *testing.T) {
	board := &config.Board{AzureDevOps: &config.AzureDevOpsConfig{}}

	ApplyBoardLayout(board, storiesBoardDetail())

	wantColumns := []config.Column{
		{Name: "todo"},
		{Name: "in-progress"},
		{Name: "review"},
		{Name: "waiting-for-build"},
		{Name: "closed", Hidden: true},
	}
	if !reflect.DeepEqual(board.Columns, wantColumns) {
		t.Fatalf("Columns = %+v, want %+v", board.Columns, wantColumns)
	}
	if board.AzureDevOps.BoardColumnField != testColumnField {
		t.Fatalf("BoardColumnField = %q, want %q", board.AzureDevOps.BoardColumnField, testColumnField)
	}
	if board.AzureDevOps.BoardLevel != "Stories" {
		t.Fatalf("BoardLevel = %q, want %q", board.AzureDevOps.BoardLevel, "Stories")
	}
	review := board.AzureDevOps.BoardColumns[2]
	if review.Lane != "review" || review.Column != "Review" || review.States["Bug"] != "Active" {
		t.Fatalf("BoardColumns[2] = %+v, want review lane mapped to Review/Active", review)
	}
}

func TestApplyBoardLayoutReplacesLegacyStateMapping(t *testing.T) {
	board := &config.Board{AzureDevOps: &config.AzureDevOpsConfig{
		ColumnMap: map[string]string{"new": "New", "active": "Active"},
	}}

	ApplyBoardLayout(board, storiesBoardDetail())

	if board.AzureDevOps.ColumnMap != nil {
		t.Fatalf("ColumnMap = %v, want nil once board columns are used", board.AzureDevOps.ColumnMap)
	}
	if !board.AzureDevOps.UsesBoardColumns() {
		t.Fatal("UsesBoardColumns() = false, want true")
	}
}

func TestApplyBoardLayoutPreservesExistingHiddenFlags(t *testing.T) {
	board := &config.Board{
		AzureDevOps: &config.AzureDevOpsConfig{},
		Columns: []config.Column{
			{Name: "review", Hidden: true},
			{Name: "closed", Hidden: false},
		},
	}

	ApplyBoardLayout(board, storiesBoardDetail())

	hidden := map[string]bool{}
	for _, col := range board.Columns {
		hidden[col.Name] = col.Hidden
	}
	if !hidden["review"] {
		t.Fatal("review lane should stay hidden")
	}
	if hidden["closed"] {
		t.Fatal("closed lane should stay visible since it already existed as visible")
	}
}

func TestUniqueLaneSlugsSuffixesDuplicates(t *testing.T) {
	got := uniqueLaneSlugs([]BoardColumn{{Name: "Doing"}, {Name: "doing"}, {Name: "Done"}})
	want := []string{"doing-1", "doing-2", "done"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueLaneSlugs() = %v, want %v", got, want)
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Waiting for build": "waiting-for-build",
		"In  Progress!":     "in-progress",
		"--ToDo--":          "todo",
		"QA / Test 2":       "qa-test-2",
	}
	for in, want := range tests {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
