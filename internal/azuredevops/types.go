package azuredevops

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

type CommentListResponse struct {
	Count    int       `json:"count"`
	Comments []Comment `json:"comments"`
}
