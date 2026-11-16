package azuredevops

import (
	"encoding/json"
	"fmt"
	"strings"
)

type WorkItemReference struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type WIQLResult struct {
	WorkItems []WorkItemReference `json:"workItems"`
}

// WorkItemLinkQueryResult is the response shape for a WIQL "work item links"
// (tree) query, as opposed to a flat WorkItems query. The first relation for
// each root has a nil Source; every other relation is a parent/child edge.
type WorkItemLinkQueryResult struct {
	WorkItemRelations []WorkItemLinkRelation `json:"workItemRelations"`
}

type WorkItemLinkRelation struct {
	Source *WorkItemReference `json:"source"`
	Target *WorkItemReference `json:"target"`
}

type IdentityRef struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

type Profile struct {
	ID string `json:"id"`
}

type WorkItemFields struct {
	ID            int          `json:"System.Id"`
	Title         string       `json:"System.Title"`
	State         string       `json:"System.State"`
	Description   string       `json:"System.Description"`
	ReproSteps    string       `json:"Microsoft.VSTS.TCM.ReproSteps"`
	Tags          string       `json:"System.Tags"`
	WorkItemType  string       `json:"System.WorkItemType"`
	AssignedTo    *IdentityRef `json:"System.AssignedTo"`
	IterationPath string       `json:"System.IterationPath"`
	ChangedDate   string       `json:"System.ChangedDate"`
	CreatedDate   string       `json:"System.CreatedDate"`
	Parent        int          `json:"System.Parent"`
	BoardColumn   string       `json:"System.BoardColumn"`
	CommentCount  int          `json:"System.CommentCount"`
	// KanbanColumns holds each team board's column field (WEF_<id>_Kanban.Column)
	// by reference name. System.BoardColumn only reflects the board of the
	// team owning the item's area path, which differs from the configured
	// board for items in other teams' areas.
	KanbanColumns map[string]string `json:"-"`
}

const kanbanColumnFieldSuffix = "_Kanban.Column"

func (f *WorkItemFields) UnmarshalJSON(data []byte) error {
	type plainFields WorkItemFields
	if err := json.Unmarshal(data, (*plainFields)(f)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for name, value := range raw {
		if !strings.HasSuffix(name, kanbanColumnFieldSuffix) {
			continue
		}
		var column string
		if err := json.Unmarshal(value, &column); err != nil {
			return fmt.Errorf("decoding %s: %w", name, err)
		}
		if f.KanbanColumns == nil {
			f.KanbanColumns = make(map[string]string)
		}
		f.KanbanColumns[name] = column
	}
	return nil
}

// boardColumn is the item's column on the board whose column field is
// columnField, or System.BoardColumn when that field wasn't fetched.
func (f WorkItemFields) boardColumn(columnField string) string {
	if column, ok := f.KanbanColumns[columnField]; ok {
		return column
	}
	return f.BoardColumn
}

type WorkItem struct {
	ID     int            `json:"id"`
	Fields WorkItemFields `json:"fields"`
}

type WorkItemListResponse struct {
	Count int        `json:"count"`
	Value []WorkItem `json:"value"`
}

type IterationAttrs struct {
	StartDate  string `json:"startDate"`
	FinishDate string `json:"finishDate"`
	TimeFrame  string `json:"timeFrame"`
}

type Iteration struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	Attributes IterationAttrs `json:"attributes"`
}

type IterationListResponse struct {
	Count int         `json:"count"`
	Value []Iteration `json:"value"`
}

// TeamSettings holds the team's default iteration, whose path is relative to
// the project (e.g. `\Team\Continuous`).
type TeamSettings struct {
	DefaultIteration Iteration `json:"defaultIteration"`
}

type TeamFieldValues struct {
	DefaultValue string `json:"defaultValue"`
}

type ConnectionData struct {
	AuthenticatedUser struct {
		ID         string `json:"id"`
		Properties struct {
			Account struct {
				Value string `json:"$value"`
			} `json:"Account"`
		} `json:"properties"`
	} `json:"authenticatedUser"`
}

// NewWorkItem is everything set on a work item created from cmdban.
type NewWorkItem struct {
	Type          string
	Title         string
	IterationPath string
	AreaPath      string
	AssignedTo    string
	Placement     Placement
}

func (w NewWorkItem) patchOperations() []PatchOperation {
	ops := []PatchOperation{
		{Op: "add", Path: "/fields/System.Title", Value: w.Title},
		{Op: "add", Path: "/fields/System.IterationPath", Value: w.IterationPath},
		{Op: "add", Path: "/fields/System.AreaPath", Value: w.AreaPath},
		{Op: "add", Path: "/fields/System.AssignedTo", Value: w.AssignedTo},
	}
	return append(ops, w.Placement.patchOperations()...)
}

const (
	fieldTags = "System.Tags"
	// tagSeparator splits System.Tags; ADO writes "; " between tags and
	// accepts ";" with or without the space on input.
	tagSeparator       = ";"
	tagOutputSeparator = tagSeparator + " "
)

func parseTags(raw string) []string {
	var tags []string
	for _, tag := range strings.Split(raw, tagSeparator) {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func formatTags(tags []string) string {
	return strings.Join(tags, tagOutputSeparator)
}

// HasTag reports whether tags contains tag, ignoring case the way ADO does
// when it matches tags.
func HasTag(tags []string, tag string) bool {
	for _, existing := range tags {
		if strings.EqualFold(existing, tag) {
			return true
		}
	}
	return false
}

type PatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type KanbanBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type KanbanBoardListResponse struct {
	Count int           `json:"count"`
	Value []KanbanBoard `json:"value"`
}

const boardColumnTypeOutgoing = "outgoing"

type BoardColumn struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ColumnType    string            `json:"columnType"` // "incoming", "inProgress", "outgoing"
	StateMappings map[string]string `json:"stateMappings"`
}

type FieldReference struct {
	ReferenceName string `json:"referenceName"`
}

type BoardFields struct {
	ColumnField FieldReference `json:"columnField"`
}

// KanbanBoardDetail is a single team board with its columns and the
// board-specific WEF field that stores each work item's column.
type KanbanBoardDetail struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Columns []BoardColumn `json:"columns"`
	Fields  BoardFields   `json:"fields"`
}

// Placement is the set of field values that put a work item in a lane: the
// state alone for state-mapped boards, or state plus Kanban column.
type Placement struct {
	State       string
	ColumnField string
	Column      string
}

func (p Placement) patchOperations() []PatchOperation {
	ops := []PatchOperation{{Op: "add", Path: "/fields/System.State", Value: p.State}}
	if p.ColumnField != "" {
		ops = append(ops, PatchOperation{Op: "add", Path: "/fields/" + p.ColumnField, Value: p.Column})
	}
	return ops
}

type Account struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
}

type AccountListResponse struct {
	Count int       `json:"count"`
	Value []Account `json:"value"`
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProjectListResponse struct {
	Count int       `json:"count"`
	Value []Project `json:"value"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TeamListResponse struct {
	Count int    `json:"count"`
	Value []Team `json:"value"`
}

type Backlog struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BacklogListResponse struct {
	Count int       `json:"count"`
	Value []Backlog `json:"value"`
}

type BacklogWorkItemTarget struct {
	ID int `json:"id"`
}

type BacklogWorkItemRef struct {
	Target BacklogWorkItemTarget `json:"target"`
}

type BacklogWorkItemsResponse struct {
	WorkItems []BacklogWorkItemRef `json:"workItems"`
}

type Comment struct {
	ID          int         `json:"id"`
	Text        string      `json:"text"`
	CreatedBy   IdentityRef `json:"createdBy"`
	CreatedDate string      `json:"createdDate"`
}

type addCommentRequest struct {
	Text string `json:"text"`
}

type CommentListResponse struct {
	Count    int       `json:"count"`
	Comments []Comment `json:"comments"`
}

type WorkItemRelation struct {
	Rel string `json:"rel"`
	URL string `json:"url"`
}

type WorkItemWithRelations struct {
	ID        int                `json:"id"`
	Relations []WorkItemRelation `json:"relations"`
}

type PullRequestReviewer struct {
	DisplayName string `json:"displayName"`
	Vote        int    `json:"vote"`
}

type GitRepository struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	WebURL  string  `json:"webUrl"`
	Project Project `json:"project"`
}

type GitCommitRef struct {
	CommitID string `json:"commitId"`
}

type PullRequest struct {
	PullRequestID   int                   `json:"pullRequestId"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Status          string                `json:"status"`
	IsDraft         bool                  `json:"isDraft"`
	TargetRefName   string                `json:"targetRefName"`
	ClosedDate      string                `json:"closedDate"`
	Repository      GitRepository         `json:"repository"`
	Reviewers       []PullRequestReviewer `json:"reviewers"`
	LastMergeCommit GitCommitRef          `json:"lastMergeCommit"`
}

type PullRequestComment struct {
	CommentType string `json:"commentType"`
	IsDeleted   bool   `json:"isDeleted"`
}

type PullRequestThread struct {
	IsDeleted bool                 `json:"isDeleted"`
	Comments  []PullRequestComment `json:"comments"`
}

type PullRequestThreadListResponse struct {
	Value []PullRequestThread `json:"value"`
}

type BuildDefinitionRef struct {
	Name string `json:"name"`
}

type BuildLinks struct {
	Web struct {
		Href string `json:"href"`
	} `json:"web"`
}

type Build struct {
	ID            int                `json:"id"`
	BuildNumber   string             `json:"buildNumber"`
	Status        string             `json:"status"`
	Result        string             `json:"result"`
	SourceVersion string             `json:"sourceVersion"`
	Definition    BuildDefinitionRef `json:"definition"`
	Links         BuildLinks         `json:"_links"`
}

type BuildListResponse struct {
	Value []Build `json:"value"`
}

type PullRequestListResponse struct {
	Value []PullRequest `json:"value"`
}

// RelationPatchValue is a link added to a work item's relations.
type RelationPatchValue struct {
	Rel        string            `json:"rel"`
	URL        string            `json:"url"`
	Attributes map[string]string `json:"attributes"`
}
