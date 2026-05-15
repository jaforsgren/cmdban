package azuredevops

type WorkItemReference struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type WIQLResult struct {
	WorkItems []WorkItemReference `json:"workItems"`
}

type IdentityRef struct {
	DisplayName string `json:"displayName"`
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
