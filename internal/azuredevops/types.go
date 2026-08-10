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
	Tags          string       `json:"System.Tags"`
	WorkItemType  string       `json:"System.WorkItemType"`
	AssignedTo    *IdentityRef `json:"System.AssignedTo"`
	IterationPath string       `json:"System.IterationPath"`
	ChangedDate   string       `json:"System.ChangedDate"`
	CreatedDate   string       `json:"System.CreatedDate"`
	Parent        int          `json:"System.Parent"`
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

type BoardColumn struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ColumnType string `json:"columnType"` // "incoming", "inProgress", "outgoing"
}

type BoardColumnListResponse struct {
	Count int           `json:"count"`
	Value []BoardColumn `json:"value"`
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
