package app

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"cmdban/internal/fsutil"
)

const (
	markdownExtension = ".md"
	restoredDirMode   = 0755
	restoredFileMode  = 0644
)

// FileChangedError means a file no longer matches what history recorded, so
// applying the step would overwrite someone's newer edit.
type FileChangedError struct {
	Path string
}

func (e *FileChangedError) Error() string {
	return fmt.Sprintf("%s changed since this action", filepath.Base(e.Path))
}

// fileChange holds a file's content on each side of a command; nil means the
// file doesn't exist on that side.
type fileChange struct {
	path   string
	before []byte
	after  []byte
}

type fileCommand struct {
	label   string
	changes []fileChange
}

func (c *fileCommand) Label() string { return c.label }

func (c *fileCommand) Execute() error {
	return c.apply(func(change fileChange) ([]byte, []byte) { return change.before, change.after })
}

func (c *fileCommand) Undo() error {
	return c.apply(func(change fileChange) ([]byte, []byte) { return change.after, change.before })
}

func (c *fileCommand) apply(sides func(fileChange) (expected, target []byte)) error {
	for _, change := range c.changes {
		expected, _ := sides(change)
		current, err := readOptionalFile(change.path)
		if err != nil {
			return err
		}
		if !sameContent(current, expected) {
			return &FileChangedError{Path: change.path}
		}
	}
	for i, change := range c.changes {
		_, target := sides(change)
		if err := writeOptionalFile(change.path, target); err != nil {
			errs := []error{err}
			for _, written := range c.changes[:i] {
				expected, _ := sides(written)
				errs = append(errs, writeOptionalFile(written.path, expected))
			}
			return errors.Join(errs...)
		}
	}
	return nil
}

func sameContent(a, b []byte) bool {
	return (a == nil) == (b == nil) && bytes.Equal(a, b)
}

func readOptionalFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if content == nil {
		content = []byte{}
	}
	return content, nil
}

func writeOptionalFile(path string, content []byte) error {
	if content == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), restoredDirMode); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, content, restoredFileMode)
}

// captureMarkdownFiles reads every markdown file under directories, including
// the archive folder, so moves between them appear as paired changes.
func captureMarkdownFiles(directories []string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, directory := range directories {
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != markdownExtension {
				return nil
			}
			content, err := readOptionalFile(path)
			if err != nil {
				return err
			}
			if content != nil {
				files[path] = content
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func diffFiles(before, after map[string][]byte) []fileChange {
	var changes []fileChange
	for path, content := range before {
		if updated, ok := after[path]; !ok || !bytes.Equal(content, updated) {
			changes = append(changes, fileChange{path: path, before: content, after: updated})
		}
	}
	for path, content := range after {
		if _, ok := before[path]; !ok {
			changes = append(changes, fileChange{path: path, after: content})
		}
	}
	slices.SortFunc(changes, func(a, b fileChange) int {
		if a.path < b.path {
			return -1
		}
		if a.path > b.path {
			return 1
		}
		return 0
	})
	return changes
}
