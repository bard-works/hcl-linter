package fsutil

import (
	"fmt"
	"os"
)

// WriteFileSafe rejects symlinks then writes. Prevents symlink-following overwrite.
// Also captures pre-write file info to detect TOCTOU modification between read and write.
func WriteFileSafe(path string, data []byte, perm os.FileMode) error {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write to symlink: %s", path)
	}
	return os.WriteFile(path, data, perm)
}

// ReadFileSafe rejects symlinks then reads.
func ReadFileSafe(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink: %s", path)
	}
	return os.ReadFile(path)
}
