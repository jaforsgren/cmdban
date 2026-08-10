package azuredevops

import (
	"testing"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

func TestWorkItemToTaskCarriesParentID(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{
		ColumnMap: map[string]string{"today": "Active"},
	}
	item := WorkItem{
		Fields: WorkItemFields{
			ID:     101,
			Title:  "Child item",
			State:  "Active",
			Parent: 55,
		},
	}

	got := workItemToTask(item, cfg)

	if got.ADOParentID != 55 {
		t.Fatalf("ADOParentID = %d, want 55", got.ADOParentID)
	}
	if got.ADOItemID != 101 {
		t.Fatalf("ADOItemID = %d, want 101", got.ADOItemID)
	}
}

func TestFilterFeaturesAndStoriesDropsFeatureWithChildren(t *testing.T) {
	feature := &task.Task{ADOItemID: 1, ADOWorkItemType: workItemTypeFeature}
	story := &task.Task{ADOItemID: 2, ADOWorkItemType: workItemTypeUserStory, ADOParentID: 1}

	got := filterFeaturesAndStories([]*task.Task{feature, story})

	if len(got) != 1 || got[0] != story {
		t.Fatalf("expected only the story to remain, got %+v", got)
	}
}

func TestFilterFeaturesAndStoriesKeepsChildlessFeature(t *testing.T) {
	feature := &task.Task{ADOItemID: 1, ADOWorkItemType: workItemTypeFeature}
	unrelatedStory := &task.Task{ADOItemID: 2, ADOWorkItemType: workItemTypeUserStory, ADOParentID: 99}

	got := filterFeaturesAndStories([]*task.Task{feature, unrelatedStory})

	if len(got) != 2 {
		t.Fatalf("expected both items to remain, got %+v", got)
	}
}

func TestFilterFeaturesAndStoriesDropsOtherTypesWhenStoriesPresent(t *testing.T) {
	bug := &task.Task{ADOItemID: 1, ADOWorkItemType: "Bug"}
	story := &task.Task{ADOItemID: 2, ADOWorkItemType: workItemTypeUserStory}

	got := filterFeaturesAndStories([]*task.Task{bug, story})

	if len(got) != 1 || got[0] != story {
		t.Fatalf("expected only the story to remain, got %+v", got)
	}
}

func TestFilterFeaturesAndStoriesPassesThroughWhenNoneCarried(t *testing.T) {
	bug := &task.Task{ADOItemID: 1, ADOWorkItemType: "Bug"}
	epic := &task.Task{ADOItemID: 2, ADOWorkItemType: "Epic"}

	got := filterFeaturesAndStories([]*task.Task{bug, epic})

	if len(got) != 2 {
		t.Fatalf("expected a board with no Story/Feature items to pass through untouched, got %+v", got)
	}
}

func TestFilterForBoardModeSkipsFilterForBacklogBoards(t *testing.T) {
	epic := &task.Task{ADOItemID: 1, ADOWorkItemType: "Epic"}
	feature := &task.Task{ADOItemID: 2, ADOWorkItemType: workItemTypeFeature, ADOParentID: 1}
	story := &task.Task{ADOItemID: 3, ADOWorkItemType: workItemTypeUserStory, ADOParentID: 2}

	got := filterForBoardMode([]*task.Task{epic, feature, story}, "Epics", false)

	if len(got) != 3 {
		t.Fatalf("expected the full hierarchy to pass through untouched for a backlog board, got %+v", got)
	}
}

func TestFilterForBoardModeSkipsFilterForAssignedToMeBoards(t *testing.T) {
	feature := &task.Task{ADOItemID: 1, ADOWorkItemType: workItemTypeFeature}
	story := &task.Task{ADOItemID: 2, ADOWorkItemType: workItemTypeUserStory, ADOParentID: 1}

	got := filterForBoardMode([]*task.Task{feature, story}, "", true)

	if len(got) != 2 {
		t.Fatalf("expected assigned-to-me boards to pass through untouched, got %+v", got)
	}
}

func TestFilterForBoardModeAppliesFilterForIterationBoards(t *testing.T) {
	feature := &task.Task{ADOItemID: 1, ADOWorkItemType: workItemTypeFeature}
	story := &task.Task{ADOItemID: 2, ADOWorkItemType: workItemTypeUserStory, ADOParentID: 1}

	got := filterForBoardMode([]*task.Task{feature, story}, "", false)

	if len(got) != 1 || got[0] != story {
		t.Fatalf("expected the sprint-board collapsing to still apply, got %+v", got)
	}
}

func TestWorkItemToTaskNoParent(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{
		ColumnMap: map[string]string{"today": "Active"},
	}
	item := WorkItem{
		Fields: WorkItemFields{
			ID:    102,
			Title: "Top-level item",
			State: "Active",
		},
	}

	got := workItemToTask(item, cfg)

	if got.ADOParentID != 0 {
		t.Fatalf("ADOParentID = %d, want 0", got.ADOParentID)
	}
}
