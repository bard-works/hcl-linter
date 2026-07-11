package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileSafe_RejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	realFile := filepath.Join(tmp, "real.txt")
	if err := os.WriteFile(realFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	err := WriteFileSafe(symlink, []byte("evil"), 0o644)
	if err == nil {
		t.Fatal("expected error for symlink write")
	}
}

func TestWriteFileSafe_AllowsRegularFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file.txt")
	if err := WriteFileSafe(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected content: %s", data)
	}
}

// TestWriteFileSafe_AtomicOverwrite verifies the temp+rename path: existing
// content is replaced, custom permission bits survive the overwrite, and no
// temp-file residue is left behind.
func TestWriteFileSafe_AtomicOverwrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileSafe(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("unexpected content: %s", data)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("existing file permissions not preserved: got %v, want 0600", info.Mode().Perm())
		}
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temp-file residue left in directory: %v", entries)
	}
}

func TestWriteFileSafe_RefusesReadOnlyTarget(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not block writes when running as root")
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if err := WriteFileSafe(path, []byte("new"), 0o644); err == nil {
		t.Fatal("expected error writing to read-only file, got nil")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("read-only file was modified: %s", data)
	}
}

func TestReadFileSafe_RejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	realFile := filepath.Join(tmp, "real.txt")
	if err := os.WriteFile(realFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	_, err := ReadFileSafe(symlink)
	if err == nil {
		t.Fatal("expected error for symlink read")
	}
}

func TestReadFileSafe_AllowsRegularFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFileSafe(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected content: %s", data)
	}
}
