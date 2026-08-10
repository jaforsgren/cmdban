package azuredevops

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"cmdban/internal/config"
	"cmdban/internal/task"
)

var htmlTagRegex = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTagRegex.ReplaceAllString(s, "")
	return html.UnescapeString(strings.TrimSpace(s))
}

func StateForLane(cfg *config.AzureDevOpsConfig, lane task.Status) string {
	if state, ok := cfg.ColumnMap[string(lane)]; ok {
		return state
	}
	return string(lane)
}

func laneForState(cfg *config.AzureDevOpsConfig, state string) task.Status {
	for lane, s := range cfg.ColumnMap {
		if strings.EqualFold(s, state) {
			return task.Status(lane)
		}
	}
	return task.StatusBacklog
}

// resolveIterationPath converts a stored iteration value to the full ADO path
// required by WIQL. It handles three forms:
//   - "@CurrentIteration" → resolved via the team settings API
//   - "Project\Path\Name"  → already a full path, used as-is
//   - "Name"               → bare name, looked up from all team iterations
func resolveIterationPath(client *Client, stored string) (string, error) {
	if stored == "@CurrentIteration" {
		return client.CurrentIterationPath()
	}
	if strings.Contains(stored, `\`) {
		return stored, nil
	}
	// Bare name: find the matching iteration to get its full path.
	iterations, err := client.ListIterations()
	if err != nil {
		return "", fmt.Errorf("listing iterations: %w", err)
	}
	for _, it := range iterations {
		if strings.EqualFold(it.Name, stored) {
			return it.Path, nil
		}
	}
	return "", fmt.Errorf("iteration %q not found in team iterations", stored)
}

// resolveBacklogID looks up the backlog ID matching the configured backlog
// level name (e.g. "Epics"), since the workItems endpoint takes an ID, not
// the display name shown in the URL.
func resolveBacklogID(client *Client, backlogLevel string) (string, error) {
	backlogs, err := client.ListBacklogs()
	if err != nil {
		return "", fmt.Errorf("listing backlogs: %w", err)
	}
	for _, b := range backlogs {
		if strings.EqualFold(b.Name, backlogLevel) {
			return b.ID, nil
		}
	}
	return "", fmt.Errorf("backlog level %q not found for team", backlogLevel)
}

// fetchBacklogIDsWithDescendants returns every work item at the given
// backlog level plus all of its descendants (children, grandchildren, ...),
// so an "Epics" board also surfaces the Features and Stories beneath them.
// The descendant walk is a single recursive WIQL tree query rather than
// fetching level by level.
func fetchBacklogIDsWithDescendants(client *Client, backlogID string) ([]int, error) {
	roots, err := client.FetchBacklogWorkItemIDs(backlogID)
	if err != nil {
		return nil, err
	}
	return client.FetchDescendantIDs(roots)
}

// SyncDiagnostics reports what a FetchBoardTasks call actually did, so
// callers can explain an empty result instead of failing silently.
type SyncDiagnostics struct {
	Source        string
	IterationPath string
	FetchedCount  int
	ReturnedCount int
}

func FetchBoardTasks(adoCfg *config.AzureDevOpsConfig, pat string) ([]*task.Task, SyncDiagnostics, error) {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)

	var (
		items         []WorkItem
		iterationPath string
		source        string
		err           error
	)

	switch {
	case adoCfg.AssignedToMe:
		// Assigned-to-me boards skip iteration/backlog scoping entirely —
		// fetch everything in the project assigned to the PAT's identity,
		// mirroring "give me my work items" rather than walking a hierarchy.
		items, err = client.FetchAssignedToMeWorkItems()
		if err != nil {
			return nil, SyncDiagnostics{}, fmt.Errorf("fetching assigned work items: %w", err)
		}
		source = "assigned-to-me"

	case adoCfg.BacklogLevel != "":
		// Backlog levels list every item at that level regardless of
		// iteration — teams that only use backlogs may have no sprint
		// iterations configured at all, so iteration resolution is skipped.
		var backlogID string
		backlogID, err = resolveBacklogID(client, adoCfg.BacklogLevel)
		if err != nil {
			return nil, SyncDiagnostics{}, err
		}
		var ids []int
		ids, err = fetchBacklogIDsWithDescendants(client, backlogID)
		if err != nil {
			return nil, SyncDiagnostics{}, fmt.Errorf("fetching backlog work items: %w", err)
		}
		items, err = client.FetchWorkItemsByIDs(ids)
		if err != nil {
			return nil, SyncDiagnostics{}, fmt.Errorf("fetching work item details: %w", err)
		}
		source = "backlog:" + adoCfg.BacklogLevel

	default:
		iterationPath, err = resolveIterationPath(client, adoCfg.Iteration)
		if err != nil {
			return nil, SyncDiagnostics{}, fmt.Errorf("resolving iteration: %w", err)
		}
		items, err = client.FetchWorkItems(iterationPath)
		if err != nil {
			return nil, SyncDiagnostics{}, fmt.Errorf("fetching work items: %w", err)
		}
		source = "iteration"
	}

	tasks := make([]*task.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, workItemToTask(item, adoCfg))
	}

	filtered := filterForBoardMode(tasks, adoCfg.BacklogLevel, adoCfg.AssignedToMe)
	diag := SyncDiagnostics{
		Source:        source,
		IterationPath: iterationPath,
		FetchedCount:  len(items),
		ReturnedCount: len(filtered),
	}
	return filtered, diag, nil
}

// filterForBoardMode applies the Feature/Story collapsing filter for sprint
// (iteration-based) boards only. Backlog-cascade and assigned-to-me boards
// deliberately surface a mixed hierarchy (Epics, Features, Stories, Tasks,
// ...), so that collapsing — built for sprint boards — doesn't apply there.
func filterForBoardMode(tasks []*task.Task, backlogLevel string, assignedToMe bool) []*task.Task {
	if backlogLevel != "" || assignedToMe {
		return tasks
	}
	return filterFeaturesAndStories(tasks)
}

const (
	workItemTypeUserStory = "User Story"
	workItemTypeFeature   = "Feature"
)

// filterFeaturesAndStories keeps only User Story and Feature work items,
// collapsing a Feature down to its child User Stories when at least one is
// present in the set; a childless Feature is kept on its own. Boards that
// don't carry any Story/Feature items at all (e.g. an Epics-only backlog)
// are left untouched — the filter only applies to sprint-style boards where
// stories/features are the unit of work.
func filterFeaturesAndStories(tasks []*task.Task) []*task.Task {
	featuresWithStories := make(map[int]bool)
	hasStoryOrFeature := false
	for _, t := range tasks {
		if t.ADOWorkItemType == workItemTypeUserStory || t.ADOWorkItemType == workItemTypeFeature {
			hasStoryOrFeature = true
		}
		if t.ADOWorkItemType == workItemTypeUserStory && t.ADOParentID != 0 {
			featuresWithStories[t.ADOParentID] = true
		}
	}
	if !hasStoryOrFeature {
		return tasks
	}

	filtered := make([]*task.Task, 0, len(tasks))
	for _, t := range tasks {
		switch t.ADOWorkItemType {
		case workItemTypeUserStory:
			filtered = append(filtered, t)
		case workItemTypeFeature:
			if !featuresWithStories[t.ADOItemID] {
				filtered = append(filtered, t)
			}
		}
	}
	return filtered
}

func workItemToTask(item WorkItem, cfg *config.AzureDevOpsConfig) *task.Task {
	f := item.Fields
	lane := laneForState(cfg, f.State)
	desc := stripHTML(f.Description)

	var assignee string
	if f.AssignedTo != nil {
		assignee = f.AssignedTo.DisplayName
	}

	createdAt, _ := time.Parse(time.RFC3339, f.CreatedDate)
	updatedAt, _ := time.Parse(time.RFC3339, f.ChangedDate)

	var tags []string
	tags = append(tags, f.WorkItemType)
	if assignee != "" {
		tags = append(tags, assignee)
	}
	for _, rawTag := range strings.Split(f.Tags, ";") {
		rawTag = strings.TrimSpace(rawTag)
		if rawTag != "" {
			tags = append(tags, rawTag)
		}
	}

	return &task.Task{
		ID:              fmt.Sprintf("ado-%d", f.ID),
		Title:           f.Title,
		Description:     desc,
		Status:          lane,
		Tags:            tags,
		FilePath:        "",
		ADOItemID:       f.ID,
		ADOParentID:     f.Parent,
		ADOWorkItemType: f.WorkItemType,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
}

// FetchComments returns a work item's comments, wrapping the client's
// Client construction the same way every other read/write helper in this
// file does, so callers never construct a Client directly.
func FetchComments(adoCfg *config.AzureDevOpsConfig, pat string, workItemID int) ([]Comment, error) {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	comments, err := client.FetchComments(workItemID)
	if err != nil {
		return nil, fmt.Errorf("fetching comments for work item %d: %w", workItemID, err)
	}
	return comments, nil
}

func PushStateChange(adoCfg *config.AzureDevOpsConfig, pat string, t *task.Task, newLane task.Status) error {
	if t.ADOItemID == 0 {
		return fmt.Errorf("task %q has no ADO item ID", t.ID)
	}
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	state := StateForLane(adoCfg, newLane)
	if err := client.UpdateWorkItemState(t.ADOItemID, state); err != nil {
		return fmt.Errorf("updating work item %d state: %w", t.ADOItemID, err)
	}
	return nil
}

func UpdateRemoteTitle(adoCfg *config.AzureDevOpsConfig, pat string, t *task.Task, title string) error {
	if t.ADOItemID == 0 {
		return fmt.Errorf("task %q has no ADO item ID", t.ID)
	}
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	if err := client.UpdateWorkItemTitle(t.ADOItemID, title); err != nil {
		return fmt.Errorf("updating work item %d title: %w", t.ADOItemID, err)
	}
	return nil
}

// UpdateRemoteTitleAndDescription pushes an edited title and plain-text
// description back to the work item, htmlizing the description since ADO
// stores it as HTML (mirrors stripHTML on the read side).
func UpdateRemoteTitleAndDescription(adoCfg *config.AzureDevOpsConfig, pat string, t *task.Task, title, description string) error {
	if t.ADOItemID == 0 {
		return fmt.Errorf("task %q has no ADO item ID", t.ID)
	}
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	if err := client.UpdateWorkItemTitleAndDescription(t.ADOItemID, title, htmlizeText(description)); err != nil {
		return fmt.Errorf("updating work item %d title and description: %w", t.ADOItemID, err)
	}
	return nil
}

func htmlizeText(s string) string {
	escaped := html.EscapeString(s)
	return strings.ReplaceAll(escaped, "\n", "<br>\n")
}

func CreateRemoteTask(adoCfg *config.AzureDevOpsConfig, pat string, title string, lane task.Status) (*task.Task, error) {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)

	iterationPath, err := resolveIterationPath(client, adoCfg.Iteration)
	if err != nil {
		return nil, fmt.Errorf("resolving iteration: %w", err)
	}

	workItemType := adoCfg.DefaultWorkItemType
	if workItemType == "" {
		workItemType = "User Story"
	}

	state := StateForLane(adoCfg, lane)
	item, err := client.CreateWorkItem(workItemType, title, state, iterationPath)
	if err != nil {
		return nil, err
	}

	return workItemToTask(*item, adoCfg), nil
}
