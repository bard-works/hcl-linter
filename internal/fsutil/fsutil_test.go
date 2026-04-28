package fsutil

import (
	"os"
	"path/filepath"
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
