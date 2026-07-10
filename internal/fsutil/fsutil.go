package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileSafe rejects symlinks then writes atomically: data goes to a temp
// file in the same directory (same filesystem, so the final rename cannot
// cross devices), is fsynced, and is renamed over the target. A crash or
// SIGINT mid-write can therefore never leave a truncated target file - the
// destination either has its old content or the complete new content.
// When the target already exists its permission bits are preserved, matching
// os.WriteFile semantics (perm applies only to newly created files).
func WriteFileSafe(path string, data []byte, perm os.FileMode) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write to symlink: %s", path)
		}
		perm = info.Mode().Perm()
		// Rename only needs directory permissions, so it would silently
		// replace a read-only file. Probe target writability first to keep
		// os.WriteFile's contract: a read-only target refuses modification.
		probe, probeErr := os.OpenFile(path, os.O_WRONLY, 0)
		if probeErr != nil {
			return fmt.Errorf("open %s for writing: %w", path, probeErr)
		}
		if err := probe.Close(); err != nil {
			return fmt.Errorf("close %s probe handle: %w", path, err)
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename has succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpName, path, err)
	}
	return nil
}

// ReadFileSafe rejects symlinks then reads.
func ReadFileSafe(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink: %s", path)
	}
	return os.ReadFile(path)
}
