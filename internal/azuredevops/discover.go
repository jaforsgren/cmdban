package azuredevops

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CurrentUserID resolves the ADO identity ID (GUID) of the PAT's owner. It
// matches the "id" field on System.AssignedTo, so it can be used to filter
// work items down to those assigned to the current user.
func (c *Client) CurrentUserID() (string, error) {
	var profile Profile
	if err := c.get("https://app.vssps.visualstudio.com/_apis/profile/me?api-version=7.0", &profile); err != nil {
		return "", fmt.Errorf("fetching profile: %w", err)
	}
	return profile.ID, nil
}

// DiscoverOrganizations uses the ADO profile API to list all organisations
// accessible to the given PAT. Works for both all-org and single-org PATs.
// Returns an error if the token lacks the profile read scope.
func DiscoverOrganizations(pat string) ([]string, error) {
	c := &Client{
		pat:  pat,
		http: &http.Client{Timeout: 10 * time.Second},
	}

	userID, err := c.CurrentUserID()
	if err != nil {
		return nil, err
	}

	var accounts AccountListResponse
	accountsURL := fmt.Sprintf(
		"https://app.vssps.visualstudio.com/_apis/accounts?memberId=%s&api-version=7.0",
		userID,
	)
	if err := c.get(accountsURL, &accounts); err != nil {
		return nil, fmt.Errorf("fetching organisations: %w", err)
	}

	names := make([]string, len(accounts.Value))
	for i, a := range accounts.Value {
		names[i] = a.AccountName
	}
	return names, nil
}

func DiscoverProjects(org, pat string) ([]string, error) {
	c := NewClient(org, "", "", pat)
	projectsURL := fmt.Sprintf("https://dev.azure.com/%s/_apis/projects?$top=200&api-version=%s", url.PathEscape(org), apiVersion)

	var resp ProjectListResponse
	if err := c.get(projectsURL, &resp); err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}

	names := make([]string, len(resp.Value))
	for i, p := range resp.Value {
		names[i] = p.Name
	}
	return names, nil
}

func DiscoverBoardColumns(org, project, team, boardLevel, pat string) ([]BoardColumn, error) {
	c := NewClient(org, project, team, pat)

	boardsURL := fmt.Sprintf("%s/_apis/work/boards?api-version=%s", c.teamURL(), apiVersion)
	var boards KanbanBoardListResponse
	if err := c.get(boardsURL, &boards); err != nil {
		return nil, fmt.Errorf("fetching boards: %w", err)
	}
	if len(boards.Value) == 0 {
		return nil, fmt.Errorf("no boards found for team %q", team)
	}

	boardID := boards.Value[0].ID
	for _, b := range boards.Value {
		if strings.EqualFold(b.Name, boardLevel) {
			boardID = b.ID
			break
		}
	}

	columnsURL := fmt.Sprintf(
		"%s/_apis/work/boards/%s/columns?api-version=%s",
		c.teamURL(), url.PathEscape(boardID), apiVersion,
	)
	var columns BoardColumnListResponse
	if err := c.get(columnsURL, &columns); err != nil {
		return nil, fmt.Errorf("fetching board columns: %w", err)
	}

	return columns.Value, nil
}

func DiscoverIterations(org, project, team, pat string) ([]Iteration, error) {
	c := NewClient(org, project, team, pat)
	iterations, err := c.ListIterations()
	if err != nil {
		return nil, fmt.Errorf("fetching iterations: %w", err)
	}
	return iterations, nil
}

func DiscoverBacklogs(org, project, team, pat string) ([]Backlog, error) {
	c := NewClient(org, project, team, pat)
	backlogs, err := c.ListBacklogs()
	if err != nil {
		return nil, fmt.Errorf("fetching backlogs: %w", err)
	}
	return backlogs, nil
}

func DiscoverTeams(org, project, pat string) ([]string, error) {
	c := NewClient(org, project, "", pat)
	teamsURL := fmt.Sprintf(
		"https://dev.azure.com/%s/_apis/projects/%s/teams?$top=200&api-version=%s",
		url.PathEscape(org), url.PathEscape(project), apiVersion,
	)

	var resp TeamListResponse
	if err := c.get(teamsURL, &resp); err != nil {
		return nil, fmt.Errorf("fetching teams: %w", err)
	}

	names := make([]string, len(resp.Value))
	for i, t := range resp.Value {
		names[i] = t.Name
	}
	return names, nil
}
