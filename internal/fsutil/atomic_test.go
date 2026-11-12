package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.md")

	if err := WriteFileAtomic(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("expected %q, got %q", "hello", content)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0644 {
		t.Errorf("expected mode 0644, got %v", info.Mode().Perm())
	}
}

func TestWriteFileAtomicReplacesContentAndKeepsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.md")
	if err := os.WriteFile(path, []byte("old content that is longer"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(path, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	if string(content) != "new" {
		t.Errorf("expected %q, got %q", "new", content)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected existing mode 0600 to be kept, got %v", info.Mode().Perm())
	}
}

func TestWriteFileAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.md")

	if err := WriteFileAtomic(path, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "task.md" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only task.md in dir, got %v", names)
	}
}

func TestWriteFileAtomicWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	link := filepath.Join(dir, "link.md")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if err := WriteFileAtomic(link, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected link.md to remain a symlink")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "new" {
		t.Errorf("expected target to contain %q, got %q", "new", content)
	}
}

func TestWriteFileAtomicMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "task.md")

	if err := WriteFileAtomic(path, []byte("x"), 0644); err == nil {
		t.Error("expected an error when the directory does not exist")
	}
}
