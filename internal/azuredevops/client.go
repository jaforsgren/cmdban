package azuredevops

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiVersion = "7.0"

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
	return fmt.Sprintf("https://dev.azure.com/%s/%s", c.org, c.project)
}

func (c *Client) get(url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ADO %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return json.Unmarshal(body, out)
}

func (c *Client) post(url string, payload, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ADO %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return json.Unmarshal(body, out)
}

func (c *Client) patchJSON(url, contentType string, payload, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ADO %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

func (c *Client) CurrentIterationPath() (string, error) {
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/%s/_apis/work/teamsettings/iterations?$timeframe=current&api-version=%s",
		c.org, c.project, c.team, apiVersion,
	)

	var result IterationListResponse
	if err := c.get(url, &result); err != nil {
		return "", err
	}
	if len(result.Value) == 0 {
		return "", fmt.Errorf("no current iteration found for team %q", c.team)
	}
	return result.Value[0].Path, nil
}

func (c *Client) FetchWorkItems(iterationPath string) ([]WorkItem, error) {
	wiqlURL := fmt.Sprintf("%s/_apis/wit/wiql?api-version=%s", c.baseURL(), apiVersion)

	escapedPath := strings.ReplaceAll(iterationPath, `\`, `\\`)
	query := fmt.Sprintf(
		"SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = '%s' AND [System.IterationPath] = '%s' ORDER BY [System.ChangedDate] DESC",
		c.project, escapedPath,
	)

	var wiqlResult WIQLResult
	if err := c.post(wiqlURL, map[string]string{"query": query}, &wiqlResult); err != nil {
		return nil, err
	}

	if len(wiqlResult.WorkItems) == 0 {
		return nil, nil
	}

	ids := make([]string, len(wiqlResult.WorkItems))
	for i, ref := range wiqlResult.WorkItems {
		ids[i] = fmt.Sprintf("%d", ref.ID)
	}

	fields := "System.Id,System.Title,System.State,System.Description,System.Tags,System.WorkItemType,System.AssignedTo,System.IterationPath,System.ChangedDate,System.CreatedDate"
	detailURL := fmt.Sprintf(
		"%s/_apis/wit/workitems?ids=%s&fields=%s&api-version=%s",
		c.baseURL(), strings.Join(ids, ","), fields, apiVersion,
	)

	var listResp WorkItemListResponse
	if err := c.get(detailURL, &listResp); err != nil {
		return nil, err
	}

	return listResp.Value, nil
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

func (c *Client) CreateWorkItem(workItemType, title, state, iterationPath string) (*WorkItem, error) {
	encodedType := strings.ReplaceAll(workItemType, " ", "%20")
	url := fmt.Sprintf("%s/_apis/wit/workitems/$%s?api-version=%s", c.baseURL(), encodedType, apiVersion)

	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: title},
		{Op: "add", Path: "/fields/System.State", Value: state},
		{Op: "add", Path: "/fields/System.IterationPath", Value: iterationPath},
	}

	var item WorkItem
	if err := c.patchJSON(url, "application/json-patch+json", ops, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
