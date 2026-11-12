package azuredevops

import (
	"reflect"
	"testing"
)

func TestParsePullRequestRefsHandlesBothSlashEscapes(t *testing.T) {
	relations := []WorkItemRelation{
		{Rel: relationArtifactLink, URL: "vstfs:///Git/PullRequestId/proj%2frepo%2f3261"},
		{Rel: relationArtifactLink, URL: "vstfs:///Git/PullRequestId/proj%2Frepo%2F3265"},
		{Rel: relationArtifactLink, URL: "vstfs:///Git/Commit/proj%2frepo%2fabc123"},
		{Rel: "System.LinkTypes.Hierarchy-Reverse", URL: "https://dev.azure.com/org/_apis/wit/workItems/1"},
		{Rel: relationArtifactLink, URL: "vstfs:///Git/PullRequestId/proj%2frepo%2fnot-a-number"},
	}

	got := parsePullRequestRefs(relations)

	want := []PullRequestRef{
		{ProjectID: "proj", RepositoryID: "repo", ID: 3261},
		{ProjectID: "proj", RepositoryID: "repo", ID: 3265},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePullRequestRefs() = %+v, want %+v", got, want)
	}
}

func TestCountReviewerVotes(t *testing.T) {
	approvals, rejections := countReviewerVotes([]PullRequestReviewer{
		{Vote: reviewerVoteApproved},
		{Vote: reviewerVoteApprovedWithSuggestions},
		{Vote: 0},
		{Vote: -5},
		{Vote: reviewerVoteRejected},
	})

	if approvals != 2 || rejections != 1 {
		t.Fatalf("countReviewerVotes() = (%d, %d), want (2, 1)", approvals, rejections)
	}
}

func TestCountPullRequestCommentsSkipsSystemAndDeleted(t *testing.T) {
	threads := []PullRequestThread{
		{Comments: []PullRequestComment{{CommentType: "text"}, {CommentType: "text", IsDeleted: true}}},
		{Comments: []PullRequestComment{{CommentType: commentTypeSystem}}},
		{IsDeleted: true, Comments: []PullRequestComment{{CommentType: "text"}}},
		{Comments: []PullRequestComment{{CommentType: "text"}, {CommentType: "text"}}},
	}

	if got := countPullRequestComments(threads); got != 3 {
		t.Fatalf("countPullRequestComments() = %d, want 3", got)
	}
}

func TestPipelineRunsForCommitKeepsOnlyMergeCommitBuilds(t *testing.T) {
	merged := Build{BuildNumber: "1.0", Status: "completed", Result: "succeeded", SourceVersion: "abc"}
	merged.Definition.Name = "CI"
	merged.Links.Web.Href = "https://example/build/1"
	other := Build{BuildNumber: "1.1", SourceVersion: "def"}

	got := pipelineRunsForCommit([]Build{merged, other}, "abc")

	want := []PipelineRun{{Name: "CI", Number: "1.0", Status: "completed", Result: "succeeded", URL: "https://example/build/1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pipelineRunsForCommit() = %+v, want %+v", got, want)
	}
}

func TestPullRequestWebURL(t *testing.T) {
	pr := PullRequest{PullRequestID: 42, Repository: GitRepository{WebURL: "https://dev.azure.com/org/proj/_git/repo"}}

	if got, want := pullRequestWebURL(pr), "https://dev.azure.com/org/proj/_git/repo/pullrequest/42"; got != want {
		t.Fatalf("pullRequestWebURL() = %q, want %q", got, want)
	}
}

func TestPullRequestURLUsesPRProject(t *testing.T) {
	c := NewClient("org", "board-project", "team", "pat")

	got := c.pullRequestURL(PullRequestRef{ProjectID: "pr-project", RepositoryID: "repo", ID: 7})

	if want := "https://dev.azure.com/org/pr-project/_apis/git/repositories/repo/pullrequests/7"; got != want {
		t.Fatalf("pullRequestURL() = %q, want %q", got, want)
	}
}

func TestPullRequestArtifactURLRoundTrips(t *testing.T) {
	ref := PullRequestRef{ProjectID: "proj", RepositoryID: "repo", ID: 99}

	got := parsePullRequestRefs([]WorkItemRelation{{Rel: relationArtifactLink, URL: pullRequestArtifactURL(ref)}})

	if len(got) != 1 || got[0] != ref {
		t.Fatalf("round trip = %+v, want %+v", got, ref)
	}
}

func testPullRequest(id int, title string) PullRequest {
	return PullRequest{
		PullRequestID: id,
		Title:         title,
		Status:        "active",
		Repository:    GitRepository{ID: "repo", Name: "backend", Project: Project{ID: "proj"}},
	}
}

func TestLinkablePullRequestsExcludesLinkedAndSortsNewestFirst(t *testing.T) {
	prs := []PullRequest{testPullRequest(10, "old"), testPullRequest(30, "new"), testPullRequest(20, "linked")}

	got := linkablePullRequests(prs, []PullRequestRef{{ID: 20}})

	if len(got) != 2 || got[0].Ref.ID != 30 || got[1].Ref.ID != 10 {
		t.Fatalf("linkablePullRequests() = %+v, want PRs 30 then 10", got)
	}
	if got[0].Ref != (PullRequestRef{ProjectID: "proj", RepositoryID: "repo", ID: 30}) || got[0].Repository != "backend" {
		t.Fatalf("linkablePullRequests()[0] = %+v, want its ref and repository carried", got[0])
	}
}

func TestLinkablePullRequestsCapsList(t *testing.T) {
	prs := make([]PullRequest, myPullRequestsShown+5)
	for i := range prs {
		prs[i] = testPullRequest(i+1, "pr")
	}

	if got := linkablePullRequests(prs, nil); len(got) != myPullRequestsShown {
		t.Fatalf("len = %d, want %d", len(got), myPullRequestsShown)
	}
}
