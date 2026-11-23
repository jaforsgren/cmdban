package azuredevops

import (
	"fmt"
	"reflect"
	"strings"

	"cmdban/internal/app"
	"cmdban/internal/config"
	"cmdban/internal/task"
)

// Provider is one ADO board's connection, resolving its PAT on each call so a
// token added through :pats applies without rebuilding the provider.
type Provider struct {
	config      *config.AzureDevOpsConfig
	lookupToken TokenLookup
	logSync     SyncLogger
}

func NewProvider(adoCfg *config.AzureDevOpsConfig, lookupToken TokenLookup, logSync SyncLogger) *Provider {
	return &Provider{config: adoCfg, lookupToken: lookupToken, logSync: logSync}
}

func (p *Provider) token() (string, error) {
	token, ok := p.lookupToken(p.config.PAT)
	if !ok {
		return "", &PATNotFoundError{Name: p.config.PAT}
	}
	return token, nil
}

func (p *Provider) Pull() ([]*task.Task, error) {
	token, err := p.token()
	if err != nil {
		return nil, err
	}
	tasks, diag, err := FetchBoardTasks(p.config, token)
	if err != nil {
		return nil, err
	}
	p.logSync(fmt.Sprintf(
		"ADO sync %s/%s/%s source=%s iteration=%q: fetched %d, showing %d",
		p.config.Org, p.config.Project, p.config.Team, diag.Source, diag.IterationPath, diag.FetchedCount, diag.ReturnedCount,
	))
	return tasks, nil
}

func (p *Provider) Create(title, workItemType string, lane task.Status) (*task.Task, error) {
	token, err := p.token()
	if err != nil {
		return nil, err
	}
	return CreateRemoteTask(p.config, token, title, workItemType, lane)
}

func (p *Provider) Move(t *task.Task, lane task.Status) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return PushStateChange(p.config, token, t, lane)
}

func (p *Provider) UpdateTitle(t *task.Task, title string) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return UpdateRemoteTitle(p.config, token, t, title)
}

func (p *Provider) UpdateTitleAndDescription(t *task.Task, title, description string) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return UpdateRemoteTitleAndDescription(p.config, token, t, title, description)
}

func (p *Provider) AddTag(t *task.Task, tag string) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return AddRemoteTag(p.config, token, t, tag)
}

func (p *Provider) undoableFields() []string {
	fields := []string{fieldTitle, fieldDescription, fieldState, fieldTags}
	if p.config.BoardColumnField != "" {
		fields = append(fields, p.config.BoardColumnField)
	}
	return fields
}

func (p *Provider) client(token string) *Client {
	return NewClient(p.config.Org, p.config.Project, p.config.Team, token)
}

// Snapshot reads raw field values because the board's markdown is a lossy
// conversion of the HTML ADO stores.
func (p *Provider) Snapshot(t *task.Task) (app.RemoteSnapshot, error) {
	token, err := p.token()
	if err != nil {
		return app.RemoteSnapshot{}, err
	}
	client := p.client(token)
	var item rawWorkItem
	fieldsURL := client.workItemURL(t.ADOItemID) + "&fields=" + strings.Join(p.undoableFields(), ",")
	if err := client.get(fieldsURL, &item); err != nil {
		return app.RemoteSnapshot{}, fmt.Errorf("reading work item %d: %w", t.ADOItemID, err)
	}
	return app.RemoteSnapshot{Revision: item.Rev, Fields: item.Fields}, nil
}

// Restore relies on the revision test to make ADO reject the write when the
// work item changed after current was read.
func (p *Provider) Restore(t *task.Task, current, target app.RemoteSnapshot) (int, error) {
	ops := p.restoreOperations(current, target)
	if len(ops) == 0 {
		return current.Revision, nil
	}
	token, err := p.token()
	if err != nil {
		return 0, err
	}
	client := p.client(token)
	var updated rawWorkItem
	payload := append([]PatchOperation{revisionTest(current.Revision)}, ops...)
	if err := client.patchJSON(client.workItemURL(t.ADOItemID), "application/json-patch+json", payload, &updated); err != nil {
		return 0, fmt.Errorf("restoring work item %d (it may have changed in Azure DevOps): %w", t.ADOItemID, err)
	}
	return updated.Rev, nil
}

func (p *Provider) restoreOperations(current, target app.RemoteSnapshot) []PatchOperation {
	var ops []PatchOperation
	for _, field := range p.undoableFields() {
		targetValue, hasTarget := target.Fields[field]
		currentValue, hasCurrent := current.Fields[field]
		switch {
		case hasTarget && (!hasCurrent || !reflect.DeepEqual(currentValue, targetValue)):
			ops = append(ops, PatchOperation{Op: "add", Path: "/fields/" + field, Value: targetValue})
		case !hasTarget && hasCurrent:
			ops = append(ops, PatchOperation{Op: "remove", Path: "/fields/" + field})
		}
	}
	return ops
}

func (p *Provider) FetchComments(workItemID int) ([]Comment, error) {
	token, err := p.token()
	if err != nil {
		return nil, err
	}
	return FetchComments(p.config, token, workItemID)
}

func (p *Provider) AddComment(workItemID int, text string) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return AddComment(p.config, token, workItemID, text)
}

func (p *Provider) SaveCheckboxChanges(workItemID int, changes []task.CheckboxChange) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return SaveCheckboxChanges(p.config, token, workItemID, changes)
}

func (p *Provider) FetchLinkedPullRequests(workItemID int) ([]PullRequestSummary, error) {
	token, err := p.token()
	if err != nil {
		return nil, err
	}
	return FetchLinkedPullRequests(p.config, token, workItemID)
}

func (p *Provider) FetchLinkedPullRequestLinks(workItemID int) ([]PullRequestLink, error) {
	token, err := p.token()
	if err != nil {
		return nil, err
	}
	return FetchLinkedPullRequestLinks(p.config, token, workItemID)
}

func (p *Provider) FetchMyLinkablePullRequests(workItemID int) ([]LinkablePullRequest, error) {
	token, err := p.token()
	if err != nil {
		return nil, err
	}
	return FetchMyLinkablePullRequests(p.config, token, workItemID)
}

func (p *Provider) LinkPullRequest(workItemID int, ref PullRequestRef) error {
	token, err := p.token()
	if err != nil {
		return err
	}
	return LinkPullRequest(p.config, token, workItemID, ref)
}
