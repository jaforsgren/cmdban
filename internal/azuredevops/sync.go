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

func FetchBoardTasks(adoCfg *config.AzureDevOpsConfig, pat string) ([]*task.Task, error) {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)

	iterationPath, err := resolveIterationPath(client, adoCfg.Iteration)
	if err != nil {
		return nil, fmt.Errorf("resolving iteration: %w", err)
	}

	items, err := client.FetchWorkItems(iterationPath)
	if err != nil {
		return nil, fmt.Errorf("fetching work items: %w", err)
	}

	tasks := make([]*task.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, workItemToTask(item, adoCfg))
	}
	return tasks, nil
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
		ID:          fmt.Sprintf("ado-%d", f.ID),
		Title:       f.Title,
		Description: desc,
		Status:      lane,
		Tags:        tags,
		FilePath:    "",
		ADOItemID:   f.ID,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

func PushStateChange(adoCfg *config.AzureDevOpsConfig, pat string, t *task.Task, newLane task.Status) error {
	if t.ADOItemID == 0 {
		return fmt.Errorf("task %q has no ADO item ID", t.ID)
	}
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	state := StateForLane(adoCfg, newLane)
	return client.UpdateWorkItemState(t.ADOItemID, state)
}

func UpdateRemoteTitle(adoCfg *config.AzureDevOpsConfig, pat string, t *task.Task, title string) error {
	if t.ADOItemID == 0 {
		return fmt.Errorf("task %q has no ADO item ID", t.ID)
	}
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	return client.UpdateWorkItemTitle(t.ADOItemID, title)
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
