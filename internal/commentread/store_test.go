package commentread

import (
	"os"
	"path/filepath"
	"testing"
)

const testOrg = "org"

func TestObserveTreatsFirstSightingAsRead(t *testing.T) {
	s := New("")

	if !s.Observe(testOrg, 1, 3) {
		t.Fatal("Observe() = false for a new work item, want true")
	}
	if s.HasUnread(testOrg, 1, 3) {
		t.Fatal("HasUnread() = true right after first sighting, want false")
	}
}

func TestNewCommentsAfterObserveAreUnread(t *testing.T) {
	s := New("")
	s.Observe(testOrg, 1, 3)

	if s.Observe(testOrg, 1, 4) {
		t.Fatal("Observe() = true for a grown count, want the seen count kept")
	}
	if !s.HasUnread(testOrg, 1, 4) {
		t.Fatal("HasUnread() = false with a new comment, want true")
	}
}

func TestMarkReadClearsUnread(t *testing.T) {
	s := New("")
	s.Observe(testOrg, 1, 3)

	if !s.MarkRead(testOrg, 1, 4) {
		t.Fatal("MarkRead() = false for a new count, want true")
	}
	if s.MarkRead(testOrg, 1, 4) {
		t.Fatal("MarkRead() = true for an unchanged count, want false")
	}
	if s.HasUnread(testOrg, 1, 4) {
		t.Fatal("HasUnread() = true after MarkRead, want false")
	}
}

func TestObserveLowersSeenCountAfterDeletion(t *testing.T) {
	s := New("")
	s.Observe(testOrg, 1, 3)

	if !s.Observe(testOrg, 1, 2) {
		t.Fatal("Observe() = false for a dropped count, want true")
	}
	if !s.HasUnread(testOrg, 1, 3) {
		t.Fatal("HasUnread() = false for a comment added after a deletion, want true")
	}
}

func TestUnknownWorkItemHasNoUnread(t *testing.T) {
	if New("").HasUnread(testOrg, 1, 5) {
		t.Fatal("HasUnread() = true for a never observed work item, want false")
	}
}

func TestWorkItemsAreScopedByOrg(t *testing.T) {
	s := New("")
	s.Observe(testOrg, 1, 1)

	if s.HasUnread("other", 1, 5) {
		t.Fatal("HasUnread() leaked read state across orgs")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), storeFileName)
	s := New(path)
	s.Observe(testOrg, 1, 2)
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !loaded.HasUnread(testOrg, 1, 3) || loaded.HasUnread(testOrg, 1, 2) {
		t.Fatalf("loaded Seen = %v, want work item 1 seen at 2", loaded.Seen)
	}
}

func TestLoadMissingFileReturnsEmptyStore(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), storeFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(s.Seen) != 0 {
		t.Fatalf("Seen = %v, want empty", s.Seen)
	}
}

func TestLoadCorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), storeFileName)
	if err := os.WriteFile(path, []byte("seen: [not a map"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil for a corrupt file, want an error")
	}
}
