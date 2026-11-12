package azuredevops

import (
	"testing"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

func TestPushStateChangeRejectsTaskWithoutADOItemID(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{Org: "org", Project: "proj", Team: "team"}
	tk := &task.Task{ID: "local-1"}

	err := PushStateChange(cfg, "pat", tk, task.StatusToday)
	if err == nil {
		t.Fatal("PushStateChange() error = nil, want an error for a task with no ADO item ID")
	}
}

func TestUpdateRemoteTitleRejectsTaskWithoutADOItemID(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{Org: "org", Project: "proj", Team: "team"}
	tk := &task.Task{ID: "local-1"}

	err := UpdateRemoteTitle(cfg, "pat", tk, "new title")
	if err == nil {
		t.Fatal("UpdateRemoteTitle() error = nil, want an error for a task with no ADO item ID")
	}
}

func TestUpdateRemoteTitleAndDescriptionRejectsTaskWithoutADOItemID(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{Org: "org", Project: "proj", Team: "team"}
	tk := &task.Task{ID: "local-1"}

	err := UpdateRemoteTitleAndDescription(cfg, "pat", tk, "new title", "new description")
	if err == nil {
		t.Fatal("UpdateRemoteTitleAndDescription() error = nil, want an error for a task with no ADO item ID")
	}
}

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

func boardColumnsConfig() *config.AzureDevOpsConfig {
	board := &config.Board{AzureDevOps: &config.AzureDevOpsConfig{}}
	ApplyBoardLayout(board, storiesBoardDetail())
	return board.AzureDevOps
}

func TestLaneForWorkItemUsesBoardColumn(t *testing.T) {
	cfg := boardColumnsConfig()

	got := laneForWorkItem(cfg, WorkItemFields{WorkItemType: "Bug", State: "Active", BoardColumn: "Review"})

	if got != "review" {
		t.Fatalf("laneForWorkItem() = %q, want %q", got, "review")
	}
}

func TestLaneForWorkItemFallsBackToStateMappingOffBoard(t *testing.T) {
	cfg := boardColumnsConfig()

	got := laneForWorkItem(cfg, WorkItemFields{WorkItemType: "User Story", State: "Resolved"})

	if got != "waiting-for-build" {
		t.Fatalf("laneForWorkItem() = %q, want %q", got, "waiting-for-build")
	}
}

func TestLaneForWorkItemFallsBackToFirstLaneForUnmappedType(t *testing.T) {
	cfg := boardColumnsConfig()

	got := laneForWorkItem(cfg, WorkItemFields{WorkItemType: "Task", State: "Done"})

	if got != "todo" {
		t.Fatalf("laneForWorkItem() = %q, want %q", got, "todo")
	}
}

func TestLaneForWorkItemUsesStateMapForLegacyBoards(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{ColumnMap: map[string]string{"active": "Active"}}

	got := laneForWorkItem(cfg, WorkItemFields{State: "Active", BoardColumn: "Review"})

	if got != "active" {
		t.Fatalf("laneForWorkItem() = %q, want %q", got, "active")
	}
}

func TestPlacementForLaneSetsColumnAndMappedState(t *testing.T) {
	cfg := boardColumnsConfig()

	got, err := placementForLane(cfg, "review", "Bug")
	if err != nil {
		t.Fatalf("placementForLane() error = %v", err)
	}

	want := Placement{State: "Active", ColumnField: testColumnField, Column: "Review"}
	if got != want {
		t.Fatalf("placementForLane() = %+v, want %+v", got, want)
	}
}

func TestPlacementForLaneRejectsTypeNotOnBoard(t *testing.T) {
	cfg := boardColumnsConfig()

	if _, err := placementForLane(cfg, "review", "Task"); err == nil {
		t.Fatal("placementForLane() error = nil, want an error for a type the board doesn't carry")
	}
}

func TestPlacementForLaneRejectsUnknownLane(t *testing.T) {
	cfg := boardColumnsConfig()

	if _, err := placementForLane(cfg, "nope", "Bug"); err == nil {
		t.Fatal("placementForLane() error = nil, want an error for an unmapped lane")
	}
}

func TestPlacementForLaneUsesStateOnlyForLegacyBoards(t *testing.T) {
	cfg := &config.AzureDevOpsConfig{ColumnMap: map[string]string{"active": "Active"}}

	got, err := placementForLane(cfg, "active", "Bug")
	if err != nil {
		t.Fatalf("placementForLane() error = %v", err)
	}

	if got != (Placement{State: "Active"}) {
		t.Fatalf("placementForLane() = %+v, want state-only placement", got)
	}
}

func TestPlacementPatchOperations(t *testing.T) {
	stateOnly := Placement{State: "Active"}.patchOperations()
	if len(stateOnly) != 1 || stateOnly[0].Path != "/fields/System.State" {
		t.Fatalf("state-only patchOperations() = %+v, want a single System.State op", stateOnly)
	}

	withColumn := Placement{State: "Active", ColumnField: testColumnField, Column: "Review"}.patchOperations()
	if len(withColumn) != 2 || withColumn[1].Path != "/fields/"+testColumnField || withColumn[1].Value != "Review" {
		t.Fatalf("board patchOperations() = %+v, want state plus column op", withColumn)
	}
}

func stateLanesConfig() *config.AzureDevOpsConfig {
	cfg := boardColumnsConfig()
	cfg.LaneMode = config.LaneModeStates
	return cfg
}

func TestLaneForWorkItemInStateModeUsesState(t *testing.T) {
	got := laneForWorkItem(stateLanesConfig(), WorkItemFields{WorkItemType: "Bug", State: "Active", BoardColumn: "Review"})

	if got != "active" {
		t.Fatalf("laneForWorkItem() = %q, want %q", got, "active")
	}
}

func TestLaneForWorkItemInStateModeFallsBackToFirstLane(t *testing.T) {
	got := laneForWorkItem(stateLanesConfig(), WorkItemFields{WorkItemType: "Task", State: "Done"})

	if got != "new" {
		t.Fatalf("laneForWorkItem() = %q, want %q", got, "new")
	}
}

func TestPlacementForLaneInStateModeSetsStateOnly(t *testing.T) {
	got, err := placementForLane(stateLanesConfig(), "resolved", "Task")
	if err != nil {
		t.Fatalf("placementForLane() error = %v", err)
	}

	if got != (Placement{State: "Resolved"}) {
		t.Fatalf("placementForLane() = %+v, want state-only Resolved", got)
	}
}

func TestPlacementForLaneInStateModeRejectsUnknownLane(t *testing.T) {
	if _, err := placementForLane(stateLanesConfig(), "review", "Bug"); err == nil {
		t.Fatal("placementForLane() error = nil, want an error for a column lane in state mode")
	}
}

func TestWorkItemToTaskCarriesCommentCount(t *testing.T) {
	item := WorkItem{Fields: WorkItemFields{ID: 7, State: "Active", CommentCount: 3}}

	got := workItemToTask(item, boardColumnsConfig())

	if got.ADOCommentCount != 3 {
		t.Fatalf("ADOCommentCount = %d, want 3", got.ADOCommentCount)
	}
}
