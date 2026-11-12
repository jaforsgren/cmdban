package azuredevops

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"cmdban/internal/config"
)

const (
	pullRequestArtifactPrefix = "vstfs:///Git/PullRequestId/"
	relationArtifactLink      = "ArtifactLink"
	pullRequestRefParts       = 3 // project, repository, pull request ID

	commentTypeSystem = "system"

	reviewerVoteApproved                = 10
	reviewerVoteApprovedWithSuggestions = 5
	reviewerVoteRejected                = -10

	pullRequestStatusCompleted = "completed"

	// pipelineQueueSlack widens the build search window so a pipeline queued
	// the instant the PR completed isn't missed by clock skew.
	pipelineQueueSlack = time.Minute
	pipelineSearchTop  = 100

	// myPullRequestsPerProject bounds each project's query; the picker shows
	// the most recent across all projects.
	myPullRequestsPerProject = 20
	myPullRequestsShown      = 30
	pullRequestStatusAll     = "all"
	pullRequestLinkName      = "Pull Request"
)

// LinkablePullRequest is one of the user's PRs offered for linking.
type LinkablePullRequest struct {
	Ref        PullRequestRef
	Title      string
	Status     string
	Repository string
}

// PullRequestRef locates a pull request linked from a work item. The project
// is the PR's own project, which can differ from the board's.
type PullRequestRef struct {
	ProjectID    string
	RepositoryID string
	ID           int
}

// PullRequestSummary is what the task side pane shows for a linked PR.
type PullRequestSummary struct {
	ID           int
	Title        string
	Description  string
	Status       string
	IsDraft      bool
	URL          string
	Approvals    int
	Rejections   int
	CommentCount int
	Pipelines    []PipelineRun
}

// PipelineRun is a build of a PR's merge commit.
type PipelineRun struct {
	Name   string
	Number string
	Status string
	Result string
	URL    string
}

// parsePullRequestRefs extracts the PR links from a work item's relations.
// The link encodes project/repository/ID with an escaped slash whose case
// varies between links.
func parsePullRequestRefs(relations []WorkItemRelation) []PullRequestRef {
	var refs []PullRequestRef
	for _, rel := range relations {
		if rel.Rel != relationArtifactLink || !strings.HasPrefix(rel.URL, pullRequestArtifactPrefix) {
			continue
		}
		decoded, err := url.PathUnescape(strings.TrimPrefix(rel.URL, pullRequestArtifactPrefix))
		if err != nil {
			continue
		}
		parts := strings.Split(decoded, "/")
		if len(parts) != pullRequestRefParts {
			continue
		}
		id, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		refs = append(refs, PullRequestRef{ProjectID: parts[0], RepositoryID: parts[1], ID: id})
	}
	return refs
}

func countReviewerVotes(reviewers []PullRequestReviewer) (approvals, rejections int) {
	for _, r := range reviewers {
		switch r.Vote {
		case reviewerVoteApproved, reviewerVoteApprovedWithSuggestions:
			approvals++
		case reviewerVoteRejected:
			rejections++
		}
	}
	return approvals, rejections
}

// countPullRequestComments counts the human comments across a PR's threads,
// skipping system messages (votes, pushes, status changes) and deletions.
func countPullRequestComments(threads []PullRequestThread) int {
	count := 0
	for _, thread := range threads {
		if thread.IsDeleted {
			continue
		}
		for _, c := range thread.Comments {
			if c.CommentType != commentTypeSystem && !c.IsDeleted {
				count++
			}
		}
	}
	return count
}

func pipelineRunsForCommit(builds []Build, commitID string) []PipelineRun {
	var runs []PipelineRun
	for _, b := range builds {
		if b.SourceVersion != commitID {
			continue
		}
		runs = append(runs, PipelineRun{
			Name:   b.Definition.Name,
			Number: b.BuildNumber,
			Status: b.Status,
			Result: b.Result,
			URL:    b.Links.Web.Href,
		})
	}
	return runs
}

// pullRequestArtifactURL builds the vstfs link ADO uses to tie a PR to a
// work item; the inverse of parsePullRequestRefs.
func pullRequestArtifactURL(ref PullRequestRef) string {
	return fmt.Sprintf("%s%s%%2F%s%%2F%d", pullRequestArtifactPrefix, ref.ProjectID, ref.RepositoryID, ref.ID)
}

// linkablePullRequests drops PRs already linked and keeps the newest first.
func linkablePullRequests(prs []PullRequest, linked []PullRequestRef) []LinkablePullRequest {
	isLinked := make(map[int]bool, len(linked))
	for _, ref := range linked {
		isLinked[ref.ID] = true
	}
	slices.SortFunc(prs, func(a, b PullRequest) int { return cmp.Compare(b.PullRequestID, a.PullRequestID) })

	var linkable []LinkablePullRequest
	for _, pr := range prs {
		if isLinked[pr.PullRequestID] {
			continue
		}
		linkable = append(linkable, LinkablePullRequest{
			Ref:        PullRequestRef{ProjectID: pr.Repository.Project.ID, RepositoryID: pr.Repository.ID, ID: pr.PullRequestID},
			Title:      pr.Title,
			Status:     pr.Status,
			Repository: pr.Repository.Name,
		})
	}
	return linkable[:min(len(linkable), myPullRequestsShown)]
}

func pullRequestWebURL(pr PullRequest) string {
	return fmt.Sprintf("%s/pullrequest/%d", pr.Repository.WebURL, pr.PullRequestID)
}

func (c *Client) orgProjectURL(projectID string) string {
	return fmt.Sprintf("https://dev.azure.com/%s/%s", url.PathEscape(c.org), url.PathEscape(projectID))
}

func (c *Client) pullRequestURL(ref PullRequestRef) string {
	return fmt.Sprintf("%s/_apis/git/repositories/%s/pullrequests/%d",
		c.orgProjectURL(ref.ProjectID), url.PathEscape(ref.RepositoryID), ref.ID)
}

func (c *Client) FetchWorkItemRelations(workItemID int) ([]WorkItemRelation, error) {
	itemURL := fmt.Sprintf("%s/_apis/wit/workitems/%d?$expand=relations&api-version=%s", c.baseURL(), workItemID, apiVersion)
	var item WorkItemWithRelations
	if err := c.get(itemURL, &item); err != nil {
		return nil, err
	}
	return item.Relations, nil
}

func (c *Client) FetchPullRequest(ref PullRequestRef) (PullRequest, error) {
	var pr PullRequest
	err := c.get(fmt.Sprintf("%s?api-version=%s", c.pullRequestURL(ref), apiVersion), &pr)
	return pr, err
}

func (c *Client) FetchPullRequestThreads(ref PullRequestRef) ([]PullRequestThread, error) {
	var resp PullRequestThreadListResponse
	err := c.get(fmt.Sprintf("%s/threads?api-version=%s", c.pullRequestURL(ref), apiVersion), &resp)
	return resp.Value, err
}

// FetchMergeCommitBuilds lists the builds of a completed PR's merge commit:
// builds of the repository's target branch queued since the PR closed,
// narrowed to the merge commit.
func (c *Client) FetchMergeCommitBuilds(ref PullRequestRef, pr PullRequest) ([]PipelineRun, error) {
	closed, err := time.Parse(time.RFC3339, pr.ClosedDate)
	if err != nil {
		return nil, fmt.Errorf("parsing PR %d closed date: %w", pr.PullRequestID, err)
	}
	query := url.Values{
		"repositoryId":   {ref.RepositoryID},
		"repositoryType": {"TfsGit"},
		"branchName":     {pr.TargetRefName},
		"minTime":        {closed.Add(-pipelineQueueSlack).UTC().Format(time.RFC3339)},
		"queryOrder":     {"queueTimeAscending"},
		"$top":           {strconv.Itoa(pipelineSearchTop)},
		"api-version":    {apiVersion},
	}
	buildsURL := fmt.Sprintf("%s/_apis/build/builds?%s", c.orgProjectURL(ref.ProjectID), query.Encode())
	var resp BuildListResponse
	if err := c.get(buildsURL, &resp); err != nil {
		return nil, err
	}
	return pipelineRunsForCommit(resp.Value, pr.LastMergeCommit.CommitID), nil
}

func (c *Client) pullRequestSummary(ref PullRequestRef) (PullRequestSummary, error) {
	pr, err := c.FetchPullRequest(ref)
	if err != nil {
		return PullRequestSummary{}, fmt.Errorf("fetching PR %d: %w", ref.ID, err)
	}
	threads, err := c.FetchPullRequestThreads(ref)
	if err != nil {
		return PullRequestSummary{}, fmt.Errorf("fetching PR %d threads: %w", ref.ID, err)
	}
	approvals, rejections := countReviewerVotes(pr.Reviewers)
	summary := PullRequestSummary{
		ID:           pr.PullRequestID,
		Title:        pr.Title,
		Description:  pr.Description,
		Status:       pr.Status,
		IsDraft:      pr.IsDraft,
		URL:          pullRequestWebURL(pr),
		Approvals:    approvals,
		Rejections:   rejections,
		CommentCount: countPullRequestComments(threads),
	}
	if pr.Status == pullRequestStatusCompleted && pr.LastMergeCommit.CommitID != "" {
		summary.Pipelines, err = c.FetchMergeCommitBuilds(ref, pr)
		if err != nil {
			return PullRequestSummary{}, fmt.Errorf("fetching PR %d pipelines: %w", ref.ID, err)
		}
	}
	return summary, nil
}

// currentIdentityID returns the PAT owner's identity ID, as used by PR search
// criteria such as creatorId.
func (c *Client) currentIdentityID() (string, error) {
	data, err := c.connectionData()
	if err != nil {
		return "", err
	}
	if data.AuthenticatedUser.ID == "" {
		return "", fmt.Errorf("could not resolve the current user's identity")
	}
	return data.AuthenticatedUser.ID, nil
}

// fetchPullRequestsCreatedBy lists a creator's PRs in every project of the
// org, since a board's work items can link PRs from other projects.
func (c *Client) fetchPullRequestsCreatedBy(creatorID string) ([]PullRequest, error) {
	projects, err := DiscoverProjects(c.org, c.pat)
	if err != nil {
		return nil, err
	}
	var all []PullRequest
	for _, project := range projects {
		query := url.Values{
			"searchCriteria.creatorId": {creatorID},
			"searchCriteria.status":    {pullRequestStatusAll},
			"$top":                     {strconv.Itoa(myPullRequestsPerProject)},
			"api-version":              {apiVersion},
		}
		prsURL := fmt.Sprintf("%s/_apis/git/pullrequests?%s", c.orgProjectURL(project), query.Encode())
		var resp PullRequestListResponse
		if err := c.get(prsURL, &resp); err != nil {
			return nil, fmt.Errorf("listing pull requests in %q: %w", project, err)
		}
		all = append(all, resp.Value...)
	}
	return all, nil
}

func (c *Client) addPullRequestLink(workItemID int, ref PullRequestRef) error {
	itemURL := fmt.Sprintf("%s/_apis/wit/workitems/%d?api-version=%s", c.baseURL(), workItemID, apiVersion)
	ops := []PatchOperation{{
		Op:   "add",
		Path: "/relations/-",
		Value: RelationPatchValue{
			Rel:        relationArtifactLink,
			URL:        pullRequestArtifactURL(ref),
			Attributes: map[string]string{"name": pullRequestLinkName},
		},
	}}
	return c.patchJSON(itemURL, "application/json-patch+json", ops, nil)
}

// FetchMyLinkablePullRequests lists the PAT owner's own PRs, newest first,
// leaving out those already linked to the work item.
func FetchMyLinkablePullRequests(adoCfg *config.AzureDevOpsConfig, pat string, workItemID int) ([]LinkablePullRequest, error) {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	creatorID, err := client.currentIdentityID()
	if err != nil {
		return nil, fmt.Errorf("resolving current user: %w", err)
	}
	prs, err := client.fetchPullRequestsCreatedBy(creatorID)
	if err != nil {
		return nil, err
	}
	relations, err := client.FetchWorkItemRelations(workItemID)
	if err != nil {
		return nil, fmt.Errorf("fetching work item %d links: %w", workItemID, err)
	}
	return linkablePullRequests(prs, parsePullRequestRefs(relations)), nil
}

// LinkPullRequest links a pull request to a work item.
func LinkPullRequest(adoCfg *config.AzureDevOpsConfig, pat string, workItemID int, ref PullRequestRef) error {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	if err := client.addPullRequestLink(workItemID, ref); err != nil {
		return fmt.Errorf("linking PR %d to work item %d: %w", ref.ID, workItemID, err)
	}
	return nil
}

// FetchLinkedPullRequests returns a summary of every PR linked to the work
// item, including pipeline runs of the merge commit for completed PRs.
func FetchLinkedPullRequests(adoCfg *config.AzureDevOpsConfig, pat string, workItemID int) ([]PullRequestSummary, error) {
	client := NewClient(adoCfg.Org, adoCfg.Project, adoCfg.Team, pat)
	relations, err := client.FetchWorkItemRelations(workItemID)
	if err != nil {
		return nil, fmt.Errorf("fetching work item %d links: %w", workItemID, err)
	}
	refs := parsePullRequestRefs(relations)
	summaries := make([]PullRequestSummary, 0, len(refs))
	for _, ref := range refs {
		summary, err := client.pullRequestSummary(ref)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}
