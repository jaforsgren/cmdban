package azuredevops

import (
	"errors"
	"testing"

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
