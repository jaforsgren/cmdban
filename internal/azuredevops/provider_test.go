package azuredevops

import (
	"errors"
	"testing"

	"cmdban/internal/app"
	"cmdban/internal/config"
)

func TestProviderReportsMissingPAT(t *testing.T) {
	const patName = "work"
	provider := NewProvider(
		&config.AzureDevOpsConfig{PAT: patName},
		func(string) (string, bool) { return "", false },
		func(string) {},
	)

	_, err := provider.Pull()

	var notFound *PATNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v, want PATNotFoundError", err)
	}
	if notFound.Name != patName {
		t.Errorf("Name = %q, want %q", notFound.Name, patName)
	}
}

func TestRestoreOperationsPatchOnlyChangedFields(t *testing.T) {
	const columnField = "WEF_Kanban.Column"
	provider := NewProvider(&config.AzureDevOpsConfig{BoardColumnField: columnField}, nil, nil)
	current := app.RemoteSnapshot{Fields: map[string]any{fieldTitle: "same", fieldState: "Active", columnField: "Doing", fieldTags: "new"}}
	target := app.RemoteSnapshot{Fields: map[string]any{fieldTitle: "same", fieldState: "New", columnField: "Todo"}}

	ops := provider.restoreOperations(current, target)

	want := map[string]string{
		"/fields/" + fieldState:  "add",
		"/fields/" + columnField: "add",
		"/fields/" + fieldTags:   "remove",
	}
	if len(ops) != len(want) {
		t.Fatalf("ops = %+v, want %d operations", ops, len(want))
	}
	for _, op := range ops {
		if want[op.Path] != op.Op {
			t.Errorf("%s op = %q, want %q", op.Path, op.Op, want[op.Path])
		}
	}
}
