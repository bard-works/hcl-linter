package rules

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindFileInAncestors_FoundAtGitRoot(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "a", "b"))
	mustWriteFile(t, filepath.Join(root, "a", "b", "terragrunt.hcl"), "terraform {}")

	found, err := findFileInAncestors(nil, filepath.Join(root, "a", "b"), "terragrunt.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != filepath.Join(root, "a", "b", "terragrunt.hcl") {
		t.Errorf("expected found in b, got %q", found)
	}
}

func TestFindFileInAncestors_StopsAtGitRoot(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "repo")
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "a", "b"))
	// Target sits above the git root; the walk must not reach it.
	mustWriteFile(t, filepath.Join(outer, "terragrunt.hcl"), "terraform {}")

	found, err := findFileInAncestors(nil, filepath.Join(root, "a", "b"), "terragrunt.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != "" {
		t.Errorf("expected not found above git root, got %q", found)
	}
}

func TestFindFileInAncestors_OpenBreakerAbortsWalk(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "a"))
	mustWriteFile(t, filepath.Join(root, "terragrunt.hcl"), "terraform {}")

	cb := NewCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	_, err := findFileInAncestors(cb, filepath.Join(root, "a"), "terragrunt.hcl")
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestAncestorWalkBoundary_GitRootWins(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "a", "b"))

	got := ancestorWalkBoundary(filepath.Join(root, "a", "b"))
	if got != root {
		t.Errorf("expected boundary %q, got %q", root, got)
	}
}

func TestAncestorWalkBoundary_NoGitNoHome(t *testing.T) {
	dir := t.TempDir()
	got := ancestorWalkBoundary(dir)
	if got != "" {
		t.Errorf("expected unbounded walk (empty boundary), got %q", got)
	}
}

func TestIsWithinDir(t *testing.T) {
	if !isWithinDir("/home/user", "/home/user") {
		t.Error("dir should be within itself")
	}
	if !isWithinDir("/home/user", "/home/user/project") {
		t.Error("descendant should be within dir")
	}
	if isWithinDir("/home/user", "/other") {
		t.Error("unrelated path should not be within dir")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
