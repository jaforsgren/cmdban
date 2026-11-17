package azuredevops

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiVersion = "7.0"

// commentsAPIVersion targets the work item comments endpoint, which is still
// preview-only as of api-version 7.x; preview.4 is the first to accept the
// format parameter.
const commentsAPIVersion = "7.1-preview.4"

// commentFormatMarkdown stores new comments as markdown, which ADO renders
// natively, so editor input is posted without conversion.
const commentFormatMarkdown = "markdown"

// defaultHost is the Azure DevOps Services origin every request goes to.
const defaultHost = "https://dev.azure.com"

type Client struct {
	host    string
	org     string
	project string
	team    string
	pat     string
	http    *http.Client
}

func NewClient(org, project, team, pat string) *Client {
	return &Client{
		host:    defaultHost,
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
	return fmt.Sprintf("%s/%s/%s", c.host, url.PathEscape(c.org), url.PathEscape(c.project))
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
	_, err := c.send(method, url, contentType, nil, payload, out)
	return err
}

// send is do for requests that need extra request headers or the response
// headers, such as the ETag guarding wiki page edits.
func (c *Client) send(method, url, contentType string, headers map[string]string, payload, out any) (http.Header, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return resp.Header, &HTTPError{Method: method, URL: url, StatusCode: resp.StatusCode, Body: truncate(string(respBody), 200)}
	}

	if out == nil {
		return resp.Header, nil
	}
	return resp.Header, json.Unmarshal(respBody, out)
}

// HTTPError is a request ADO answered with an error status.
type HTTPError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("ADO %s %s: %d %s", e.Method, e.URL, e.StatusCode, e.Body)
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
	if err := c.get(currentURL, &current); err != nil && !isNoCurrentIteration(err) {
		return "", err
	}
	if len(current.Value) > 0 {
		return current.Value[0].Path, nil
	}

	// Between sprints: fall back to the most recently started past iteration.
	// ADO's $timeframe filter only accepts "current", so filter the full list.
	iterations, err := c.ListIterations()
	if err != nil {
		return "", err
	}
	if latest, ok := latestPastIteration(iterations); ok {
		return latest.Path, nil
	}

	// Teams without sprints (e.g. continuous-flow teams) only have a default iteration.
	return c.TeamDefaultIterationPath()
}

// latestPastIteration compares start dates as strings, which is safe because
// ADO returns them as ISO 8601 UTC timestamps.
func latestPastIteration(iterations []Iteration) (Iteration, bool) {
	var latest Iteration
	found := false
	for _, it := range iterations {
		if it.Attributes.TimeFrame != TimeFramePast {
			continue
		}
		if !found || it.Attributes.StartDate > latest.Attributes.StartDate {
			latest, found = it, true
		}
	}
	return latest, found
}

// noCurrentIterationType is the exception ADO reports, as a 404 rather than
// an empty list, when no team iteration contains today's date. The
// "Exception" suffix is omitted because HTTPError truncates the body right
// before it.
const noCurrentIterationType = "CurrentIterationDoesNotExist"

func isNoCurrentIteration(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) &&
		httpErr.StatusCode == http.StatusNotFound &&
		strings.Contains(httpErr.Body, noCurrentIterationType)
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

const workItemFields = "System.Id,System.Title,System.State,System.Description,Microsoft.VSTS.TCM.ReproSteps,System.Tags,System.WorkItemType,System.AssignedTo,System.IterationPath,System.ChangedDate,System.CreatedDate,System.Parent,System.BoardColumn,System.CommentCount"

// workItemFieldList is the fields query value: the standard fields plus
// extraFields, such as a board's WEF Kanban column field.
func workItemFieldList(extraFields []string) string {
	return strings.Join(append([]string{workItemFields}, extraFields...), ",")
}

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

// escapeWIQLPath doubles backslashes, which WIQL string literals treat as
// escapes, so classification paths match literally.
func escapeWIQLPath(path string) string {
	return strings.ReplaceAll(path, `\`, `\\`)
}

func (c *Client) FetchWorkItems(iterationPath string, extraFields ...string) ([]WorkItem, error) {
	where := fmt.Sprintf("[System.TeamProject] = '%s' AND [System.IterationPath] = '%s'", c.project, escapeWIQLPath(iterationPath))
	return c.fetchWorkItemsByWIQL(where, extraFields)
}

// FetchAssignedToMeWorkItems returns full details for every work item in the
// project assigned to the PAT's own identity, via WIQL's @Me macro — no
// iteration or backlog-hierarchy scoping, just "everything that's mine".
// A non-empty unassignedAreaPath also includes unassigned items under that
// area, mirroring the web board's "@Me + Unassigned" filter.
func (c *Client) FetchAssignedToMeWorkItems(unassignedAreaPath string, extraFields ...string) ([]WorkItem, error) {
	return c.fetchWorkItemsByWIQL(assignedToMeWhereClause(c.project, unassignedAreaPath), extraFields)
}

func assignedToMeWhereClause(project, unassignedAreaPath string) string {
	if unassignedAreaPath == "" {
		return fmt.Sprintf("[System.TeamProject] = '%s' AND [System.AssignedTo] = @Me", project)
	}
	return fmt.Sprintf(
		"[System.TeamProject] = '%s' AND ([System.AssignedTo] = @Me OR ([System.AssignedTo] = '' AND [System.AreaPath] UNDER '%s'))",
		project, escapeWIQLPath(unassignedAreaPath),
	)
}

func (c *Client) fetchWorkItemsByWIQL(whereClause string, extraFields []string) ([]WorkItem, error) {
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
	return c.FetchWorkItemsByIDs(ids, extraFields...)
}

// FetchWorkItemsByIDs fetches full work item details for an explicit list of
// IDs, batching requests to stay under the API's per-call limit.
func (c *Client) FetchWorkItemsByIDs(ids []int, extraFields ...string) ([]WorkItem, error) {
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
			c.baseURL(), strings.Join(strIDs, ","), workItemFieldList(extraFields), apiVersion,
		)
		var listResp WorkItemListResponse
		if err := c.get(detailURL, &listResp); err != nil {
			return nil, err
		}
		all = append(all, listResp.Value...)
	}

	return all, nil
}

func (c *Client) workItemURL(id int) string {
	return fmt.Sprintf("%s/_apis/wit/workitems/%d?api-version=%s", c.baseURL(), id, apiVersion)
}

func (c *Client) UpdateWorkItemPlacement(id int, placement Placement) error {
	return c.patchJSON(c.workItemURL(id), "application/json-patch+json", placement.patchOperations(), nil)
}

func (c *Client) UpdateWorkItemTitle(id int, title string) error {
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: title},
	}
	return c.patchJSON(c.workItemURL(id), "application/json-patch+json", ops, nil)
}

func (c *Client) UpdateWorkItemTitleAndDescription(id int, title, description string) error {
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: title},
		{Op: "add", Path: "/fields/System.Description", Value: description},
	}
	return c.patchJSON(c.workItemURL(id), "application/json-patch+json", ops, nil)
}

// FetchWorkItemTags reads a work item's current tags straight from ADO, so a
// tag write starts from what the server has rather than a stale board sync.
func (c *Client) FetchWorkItemTags(id int) ([]string, error) {
	var item WorkItem
	if err := c.get(c.workItemURL(id)+"&fields="+fieldTags, &item); err != nil {
		return nil, err
	}
	return parseTags(item.Fields.Tags), nil
}

// UpdateWorkItemTags replaces the work item's whole tag list; ADO has no
// operation that appends a single tag.
func (c *Client) UpdateWorkItemTags(id int, tags []string) error {
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/" + fieldTags, Value: formatTags(tags)},
	}
	return c.patchJSON(c.workItemURL(id), "application/json-patch+json", ops, nil)
}

func (c *Client) CreateWorkItem(newItem NewWorkItem) (*WorkItem, error) {
	createURL := fmt.Sprintf("%s/_apis/wit/workitems/$%s?api-version=%s", c.baseURL(), url.PathEscape(newItem.Type), apiVersion)

	var item WorkItem
	if err := c.patchJSON(createURL, "application/json-patch+json", newItem.patchOperations(), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *Client) commentsURL(workItemID int) string {
	return fmt.Sprintf("%s/_apis/wit/workItems/%d/comments", c.baseURL(), workItemID)
}

func (c *Client) addCommentURL(workItemID int) string {
	return fmt.Sprintf("%s?format=%s&api-version=%s", c.commentsURL(workItemID), commentFormatMarkdown, commentsAPIVersion)
}

// AddComment posts a markdown comment to a work item.
func (c *Client) AddComment(workItemID int, text string) error {
	return c.post(c.addCommentURL(workItemID), addCommentRequest{Text: text}, nil)
}

// TeamDefaultIterationPath returns the full work item iteration path of the
// team's default iteration, used when a board has no iteration configured.
func (c *Client) TeamDefaultIterationPath() (string, error) {
	settingsURL := fmt.Sprintf("%s/_apis/work/teamsettings?api-version=%s", c.teamURL(), apiVersion)
	var settings TeamSettings
	if err := c.get(settingsURL, &settings); err != nil {
		return "", err
	}
	if settings.DefaultIteration.Path == "" {
		return "", fmt.Errorf("team %q has no default iteration", c.team)
	}
	return projectRelativePath(c.project, settings.DefaultIteration.Path), nil
}

// projectRelativePath prefixes a team-settings classification path, which
// omits the project, with the project name as work item fields expect.
func projectRelativePath(project, path string) string {
	return project + `\` + strings.TrimPrefix(path, `\`)
}

// TeamDefaultAreaPath returns the area path new work items need to appear on
// the team's boards.
func (c *Client) TeamDefaultAreaPath() (string, error) {
	areaURL := fmt.Sprintf("%s/_apis/work/teamsettings/teamfieldvalues?api-version=%s", c.teamURL(), apiVersion)
	var values TeamFieldValues
	if err := c.get(areaURL, &values); err != nil {
		return "", err
	}
	if values.DefaultValue == "" {
		return "", fmt.Errorf("team %q has no default area path", c.team)
	}
	return values.DefaultValue, nil
}

// connectionData describes the PAT owner as seen by the organisation.
func (c *Client) connectionData() (ConnectionData, error) {
	connectionURL := fmt.Sprintf("%s/%s/_apis/connectionData", c.host, url.PathEscape(c.org))
	var data ConnectionData
	err := c.get(connectionURL, &data)
	return data, err
}

// CurrentUserUniqueName returns the PAT owner's account name, the value
// System.AssignedTo accepts to assign a work item to them.
func (c *Client) CurrentUserUniqueName() (string, error) {
	data, err := c.connectionData()
	if err != nil {
		return "", err
	}
	name := data.AuthenticatedUser.Properties.Account.Value
	if name == "" {
		return "", fmt.Errorf("could not resolve the current user's account name")
	}
	return name, nil
}

// FetchComments returns a work item's comments, oldest first, with HTML
// comment text converted to markdown.
func (c *Client) FetchComments(workItemID int) ([]Comment, error) {
	url := fmt.Sprintf("%s?api-version=%s", c.commentsURL(workItemID), commentsAPIVersion)
	var resp CommentListResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}
	for i := range resp.Comments {
		resp.Comments[i].Text = htmlToMarkdown(resp.Comments[i].Text)
	}
	return resp.Comments, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
