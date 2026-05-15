package azuredevops

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DiscoverOrganizations uses the ADO profile API to list all organisations
// accessible to the given PAT. Works for both all-org and single-org PATs.
// Returns an error if the token lacks the profile read scope.
func DiscoverOrganizations(pat string) ([]string, error) {
	c := &Client{
		pat:  pat,
		http: &http.Client{Timeout: 10 * time.Second},
	}

	var profile struct {
		ID string `json:"id"`
	}
	if err := c.get("https://app.vssps.visualstudio.com/_apis/profile/me?api-version=7.0", &profile); err != nil {
		return nil, fmt.Errorf("fetching profile: %w", err)
	}

	var accounts AccountListResponse
	accountsURL := fmt.Sprintf(
		"https://app.vssps.visualstudio.com/_apis/accounts?memberId=%s&api-version=7.0",
		profile.ID,
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
	url := fmt.Sprintf("https://dev.azure.com/%s/_apis/projects?$top=200&api-version=%s", org, apiVersion)

	var resp ProjectListResponse
	if err := c.get(url, &resp); err != nil {
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

	boardsURL := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/%s/_apis/work/boards?api-version=%s",
		org, project, team, apiVersion,
	)
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
		"https://dev.azure.com/%s/%s/%s/_apis/work/boards/%s/columns?api-version=%s",
		org, project, team, boardID, apiVersion,
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

func DiscoverTeams(org, project, pat string) ([]string, error) {
	c := NewClient(org, project, "", pat)
	url := fmt.Sprintf(
		"https://dev.azure.com/%s/_apis/projects/%s/teams?$top=200&api-version=%s",
		org, project, apiVersion,
	)

	var resp TeamListResponse
	if err := c.get(url, &resp); err != nil {
		return nil, fmt.Errorf("fetching teams: %w", err)
	}

	names := make([]string, len(resp.Value))
	for i, t := range resp.Value {
		names[i] = t.Name
	}
	return names, nil
}
