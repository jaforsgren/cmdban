package azuredevops

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiVersion = "7.0"

// commentsAPIVersion targets the work item comments endpoint, which is still
// preview-only as of api-version 7.x.
const commentsAPIVersion = "7.1-preview.3"

type Client struct {
	org     string
	project string
	team    string
	pat     string
	http    *http.Client
}

func NewClient(org, project, team, pat string) *Client {
	return &Client{
		org:     org,
		project: project,
		team:    team,
		pat:     pat,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) authHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+c.pat))
}

func (c *Client) baseURL() string {
	return fmt.Sprintf("https://dev.azure.com/%s/%s", url.PathEscape(c.org), url.PathEscape(c.project))
}

// teamURL is baseURL with the team segment appended, for the team-scoped
// endpoints (iterations, backlogs).
func (c *Client) teamURL() string {
	return fmt.Sprintf("%s/%s", c.baseURL(), url.PathEscape(c.team))
}

// do executes an HTTP request against the ADO API and decodes the JSON
// response into out. A nil payload sends no body; a nil out skips decoding
// the response (used for writes that don't return anything useful).
func (c *Client) do(method, url, contentType string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ADO %s %s: %d %s", method, url, resp.StatusCode, truncate(string(respBody), 200))
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func (c *Client) get(url string, out any) error {
	return c.do(http.MethodGet, url, "", nil, out)
}

func (c *Client) post(url string, payload, out any) error {
	return c.do(http.MethodPost, url, "application/json", payload, out)
}

func (c *Client) patchJSON(url, contentType string, payload, out any) error {
	return c.do(http.MethodPatch, url, contentType, payload, out)
}

func (c *Client) ListIterations() ([]Iteration, error) {
	iterationsURL := fmt.Sprintf(
		"%s/_apis/work/teamsettings/iterations?api-version=%s",
		c.teamURL(), apiVersion,
	)
	var resp IterationListResponse
	if err := c.get(iterationsURL, &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

func (c *Client) CurrentIterationPath() (string, error) {
	currentURL := fmt.Sprintf(
		"%s/_apis/work/teamsettings/iterations?$timeframe=current&api-version=%s",
		c.teamURL(), apiVersion,
	)
	var current IterationListResponse
	if err := c.get(currentURL, &current); err != nil {
		return "", err
	}
	if len(current.Value) > 0 {
		return current.Value[0].Path, nil
	}

	// Between sprints: fall back to the most recently started past iteration.
	pastURL := fmt.Sprintf(
		"%s/_apis/work/teamsettings/iterations?$timeframe=past&api-version=%s",
		c.teamURL(), apiVersion,
	)
	var past IterationListResponse
	if err := c.get(pastURL, &past); err != nil {
		return "", err
	}
	if len(past.Value) > 0 {
		return past.Value[len(past.Value)-1].Path, nil
	}

	return "", fmt.Errorf("no iterations found for team %q", c.team)
}

func (c *Client) ListBacklogs() ([]Backlog, error) {
	backlogsURL := fmt.Sprintf(
		"%s/_apis/work/backlogs?api-version=%s",
		c.teamURL(), apiVersion,
	)
	var resp BacklogListResponse
	if err := c.get(backlogsURL, &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// FetchBacklogWorkItemIDs returns the IDs of the work items on a backlog
// level, in backlog order. Unlike sprint boards, backlog levels aren't
// scoped to a team iteration.
func (c *Client) FetchBacklogWorkItemIDs(backlogID string) ([]int, error) {
	itemsURL := fmt.Sprintf(
		"%s/_apis/work/backlogs/%s/workItems?api-version=%s",
		c.teamURL(), url.PathEscape(backlogID), apiVersion,
	)
	var resp BacklogWorkItemsResponse
	if err := c.get(itemsURL, &resp); err != nil {
		return nil, err
	}
	ids := make([]int, len(resp.WorkItems))
	for i, ref := range resp.WorkItems {
		ids[i] = ref.Target.ID
	}
	return ids, nil
}

const workItemBatchSize = 200

const workItemFields = "System.Id,System.Title,System.State,System.Description,System.Tags,System.WorkItemType,System.AssignedTo,System.IterationPath,System.ChangedDate,System.CreatedDate,System.Parent"

// FetchDescendantIDs returns the IDs of rootIDs plus every descendant
// reachable by following parent→child hierarchy links, in a single WIQL
// "work item links" tree query with MODE(Recursive) — Azure DevOps walks
// the full depth server-side, so no manual per-level fetching is needed.
func (c *Client) FetchDescendantIDs(rootIDs []int) ([]int, error) {
	if len(rootIDs) == 0 {
		return nil, nil
	}

	var all []int
	for start := 0; start < len(rootIDs); start += workItemBatchSize {
		end := min(start+workItemBatchSize, len(rootIDs))
		batch := rootIDs[start:end]
		strIDs := make([]string, len(batch))
		for i, id := range batch {
			strIDs[i] = fmt.Sprintf("%d", id)
		}

		wiqlURL := fmt.Sprintf("%s/_apis/wit/wiql?api-version=%s", c.baseURL(), apiVersion)
		query := fmt.Sprintf(
			"SELECT [System.Id] FROM WorkItemLinks WHERE "+
				"([Source].[System.TeamProject] = '%s' AND [Source].[System.Id] IN (%s)) "+
				"AND ([System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Forward') "+
				"AND ([Target].[System.TeamProject] = '%s') "+
				"MODE (Recursive)",
			c.project, strings.Join(strIDs, ","), c.project,
		)
		var result WorkItemLinkQueryResult
		if err := c.post(wiqlURL, map[string]string{"query": query}, &result); err != nil {
			return nil, err
		}
		for _, rel := range result.WorkItemRelations {
			if rel.Source != nil {
				all = append(all, rel.Source.ID)
			}
			if rel.Target != nil {
				all = append(all, rel.Target.ID)
			}
		}
	}
	return dedupeInts(all), nil
}

func dedupeInts(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func (c *Client) FetchWorkItems(iterationPath string) ([]WorkItem, error) {
	escapedPath := strings.ReplaceAll(iterationPath, `\`, `\\`)
	where := fmt.Sprintf("[System.TeamProject] = '%s' AND [System.IterationPath] = '%s'", c.project, escapedPath)
	return c.fetchWorkItemsByWIQL(where)
}

// FetchAssignedToMeWorkItems returns full details for every work item in the
// project assigned to the PAT's own identity, via WIQL's @Me macro — no
// iteration or backlog-hierarchy scoping, just "everything that's mine".
func (c *Client) FetchAssignedToMeWorkItems() ([]WorkItem, error) {
	where := fmt.Sprintf("[System.TeamProject] = '%s' AND [System.AssignedTo] = @Me", c.project)
	return c.fetchWorkItemsByWIQL(where)
}

func (c *Client) fetchWorkItemsByWIQL(whereClause string) ([]WorkItem, error) {
	wiqlURL := fmt.Sprintf("%s/_apis/wit/wiql?$top=%d&api-version=%s", c.baseURL(), workItemBatchSize, apiVersion)
	query := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE %s ORDER BY [System.ChangedDate] DESC", whereClause)

	var wiqlResult WIQLResult
	if err := c.post(wiqlURL, map[string]string{"query": query}, &wiqlResult); err != nil {
		return nil, err
	}

	if len(wiqlResult.WorkItems) == 0 {
		return nil, nil
	}

	ids := make([]int, len(wiqlResult.WorkItems))
	for i, ref := range wiqlResult.WorkItems {
		ids[i] = ref.ID
	}
	return c.FetchWorkItemsByIDs(ids)
}

// FetchWorkItemsByIDs fetches full work item details for an explicit list of
// IDs, batching requests to stay under the API's per-call limit.
func (c *Client) FetchWorkItemsByIDs(ids []int) ([]WorkItem, error) {
	var all []WorkItem
	for start := 0; start < len(ids); start += workItemBatchSize {
		end := min(start+workItemBatchSize, len(ids))
		batch := ids[start:end]
		strIDs := make([]string, len(batch))
		for i, id := range batch {
			strIDs[i] = fmt.Sprintf("%d", id)
		}
		detailURL := fmt.Sprintf(
			"%s/_apis/wit/workitems?ids=%s&fields=%s&api-version=%s",
			c.baseURL(), strings.Join(strIDs, ","), workItemFields, apiVersion,
		)
		var listResp WorkItemListResponse
		if err := c.get(detailURL, &listResp); err != nil {
			return nil, err
		}
		all = append(all, listResp.Value...)
	}

	return all, nil
}

func (c *Client) UpdateWorkItemState(id int, state string) error {
	url := fmt.Sprintf("%s/_apis/wit/workitems/%d?api-version=%s", c.baseURL(), id, apiVersion)
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.State", Value: state},
	}
	return c.patchJSON(url, "application/json-patch+json", ops, nil)
}

func (c *Client) UpdateWorkItemTitle(id int, title string) error {
	url := fmt.Sprintf("%s/_apis/wit/workitems/%d?api-version=%s", c.baseURL(), id, apiVersion)
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: title},
	}
	return c.patchJSON(url, "application/json-patch+json", ops, nil)
}

func (c *Client) UpdateWorkItemTitleAndDescription(id int, title, description string) error {
	url := fmt.Sprintf("%s/_apis/wit/workitems/%d?api-version=%s", c.baseURL(), id, apiVersion)
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: title},
		{Op: "add", Path: "/fields/System.Description", Value: description},
	}
	return c.patchJSON(url, "application/json-patch+json", ops, nil)
}

func (c *Client) CreateWorkItem(workItemType, title, state, iterationPath string) (*WorkItem, error) {
	createURL := fmt.Sprintf("%s/_apis/wit/workitems/$%s?api-version=%s", c.baseURL(), url.PathEscape(workItemType), apiVersion)

	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: title},
		{Op: "add", Path: "/fields/System.State", Value: state},
		{Op: "add", Path: "/fields/System.IterationPath", Value: iterationPath},
	}

	var item WorkItem
	if err := c.patchJSON(createURL, "application/json-patch+json", ops, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// FetchComments returns a work item's comments, oldest first, with HTML
// markup stripped from the comment text.
func (c *Client) FetchComments(workItemID int) ([]Comment, error) {
	url := fmt.Sprintf(
		"%s/_apis/wit/workItems/%d/comments?api-version=%s",
		c.baseURL(), workItemID, commentsAPIVersion,
	)
	var resp CommentListResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}
	for i := range resp.Comments {
		resp.Comments[i].Text = stripHTML(resp.Comments[i].Text)
	}
	return resp.Comments, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
