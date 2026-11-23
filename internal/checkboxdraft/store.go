package checkboxdraft

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"cmdban/internal/fsutil"
	"cmdban/internal/task"
)

const (
	storeFileName  = ".cmdban-checkbox-drafts.yaml"
	storeFilePerms = 0600
)

func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return storeFileName
	}
	return filepath.Join(home, storeFileName)
}

func New(path string) *Store {
	return &Store{Drafts: map[string][]task.CheckboxChange{}, filePath: path}
}

// Load reads the store at path; a missing file yields an empty store.
func Load(path string) (*Store, error) {
	store := New(path)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading checkbox drafts %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("parsing checkbox drafts %s: %w", path, err)
	}
	if store.Drafts == nil {
		store.Drafts = map[string][]task.CheckboxChange{}
	}
	return store, nil
}

func (s *Store) Save() error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("encoding checkbox drafts: %w", err)
	}
	return fsutil.WriteFileAtomic(s.filePath, data, storeFilePerms)
}

func (s *Store) Changes(org string, workItemID int) []task.CheckboxChange {
	return slices.Clone(s.Drafts[workItemKey(org, workItemID)])
}

// Toggle records change, or drops the pending change for that checkbox: a
// checkbox has two states, so a second toggle is back to what ADO has.
func (s *Store) Toggle(org string, workItemID int, change task.CheckboxChange) {
	changes := s.Changes(org, workItemID)
	pending := slices.IndexFunc(changes, func(c task.CheckboxChange) bool {
		return c.Number == change.Number
	})
	if pending >= 0 {
		s.Replace(org, workItemID, slices.Delete(changes, pending, pending+1))
		return
	}
	s.Replace(org, workItemID, append(changes, change))
}

func (s *Store) Replace(org string, workItemID int, changes []task.CheckboxChange) {
	key := workItemKey(org, workItemID)
	if len(changes) == 0 {
		delete(s.Drafts, key)
		return
	}
	s.Drafts[key] = changes
}

// Remove drops exactly the given changes, keeping any toggled while they were
// being saved.
func (s *Store) Remove(org string, workItemID int, saved []task.CheckboxChange) {
	remaining := slices.DeleteFunc(s.Changes(org, workItemID), func(c task.CheckboxChange) bool {
		return slices.Contains(saved, c)
	})
	s.Replace(org, workItemID, remaining)
}

// workItemKey includes the org since work item IDs are only unique within
// one Azure DevOps organization.
func workItemKey(org string, workItemID int) string {
	return fmt.Sprintf("%s/%d", org, workItemID)
}
