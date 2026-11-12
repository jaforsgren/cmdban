package commentread

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"cmdban/internal/fsutil"
)

const (
	storeFileName  = ".cmdban-comment-reads.yaml"
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
	return &Store{Seen: map[string]int{}, filePath: path}
}

// Load reads the store at path; a missing file yields an empty store.
func Load(path string) (*Store, error) {
	store := New(path)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading comment read state %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("parsing comment read state %s: %w", path, err)
	}
	if store.Seen == nil {
		store.Seen = map[string]int{}
	}
	return store, nil
}

func (s *Store) Save() error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("encoding comment read state: %w", err)
	}
	return fsutil.WriteFileAtomic(s.filePath, data, storeFilePerms)
}

// Observe records a synced work item's comment count. An item seen for the
// first time counts as read, so existing comments aren't all flagged as new;
// a count that dropped (deleted comments) lowers the seen count so the next
// new comment still shows as unread. It reports whether the store changed.
func (s *Store) Observe(org string, workItemID, commentCount int) bool {
	key := workItemKey(org, workItemID)
	seen, known := s.Seen[key]
	if known && commentCount >= seen {
		return false
	}
	s.Seen[key] = commentCount
	return true
}

// MarkRead records that the user has seen commentCount comments on the work
// item. It reports whether the store changed.
func (s *Store) MarkRead(org string, workItemID, commentCount int) bool {
	key := workItemKey(org, workItemID)
	if seen, known := s.Seen[key]; known && seen == commentCount {
		return false
	}
	s.Seen[key] = commentCount
	return true
}

func (s *Store) HasUnread(org string, workItemID, commentCount int) bool {
	seen, known := s.Seen[workItemKey(org, workItemID)]
	return known && commentCount > seen
}

// workItemKey includes the org since work item IDs are only unique within
// one Azure DevOps organization.
func workItemKey(org string, workItemID int) string {
	return fmt.Sprintf("%s/%d", org, workItemID)
}
