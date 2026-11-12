package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	tempFilePrefix = "."
	tempFileSuffix = ".tmp"
)

// WriteFileAtomic replaces path with data so readers (live reload, editors,
// sync tools) only ever see the old or the new content, never a partial
// write. The data goes to a hidden temp file in the same directory, is
// fsynced, then renamed over path; the directory is fsynced so the rename
// itself survives a crash. Symlinks are resolved first, since renaming onto
// a symlink would replace the link rather than the file it points to.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, tempFilePrefix+filepath.Base(path)+".*"+tempFileSuffix)
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()

	if err := writeAndSync(tmp, data, perm); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return syncDir(dir)
}

func writeAndSync(f *os.File, data []byte, perm os.FileMode) error {
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Chmod(perm); err != nil {
		return err
	}
	return f.Sync()
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("opening directory %s for sync: %w", dir, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("syncing directory %s: %w", dir, err)
	}
	return nil
}
