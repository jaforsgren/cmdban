package azuredevops

import "fmt"

// TokenLookup resolves a stored PAT by name.
type TokenLookup func(name string) (string, bool)

// SyncLogger receives a one-line summary of each board sync.
type SyncLogger func(summary string)

// rawWorkItem keeps field values exactly as ADO returned them.
type rawWorkItem struct {
	Rev    int            `json:"rev"`
	Fields map[string]any `json:"fields"`
}

type PATNotFoundError struct {
	Name string
}

func (e *PATNotFoundError) Error() string {
	return fmt.Sprintf("PAT %q not found — run :pats to add it", e.Name)
}
