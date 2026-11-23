package checkboxdraft

import "cmdban/internal/task"

// Store keeps checkbox toggles on Azure DevOps work items until they are
// saved, so they survive board syncs and restarts.
type Store struct {
	// Drafts maps a work item key (see workItemKey) to its unsaved changes.
	Drafts   map[string][]task.CheckboxChange `yaml:"drafts"`
	filePath string
}
