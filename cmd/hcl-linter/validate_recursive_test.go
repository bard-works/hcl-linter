package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHCL(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.hcl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateConfigRecursive_AllOK(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	goodCfg := `rules {
  block_order {
    enabled = true
    order   = ["include"]
  }
}`
	writeHCL(t, filepath.Join(root, ".hcl-linter"), goodCfg)
	writeHCL(t, filepath.Join(root, "svc", ".hcl-linter"), goodCfg)

	a.validateRecursive = true
	if err := a.runValidateConfig(nil, []string{root}); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateConfigRecursive_ReportsAllBadDirs(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	badCfg := `rules {
  blokc_order {
    enabled = true
  }
}`
	writeHCL(t, filepath.Join(root, ".hcl-linter"), badCfg)
	writeHCL(t, filepath.Join(root, "a", ".hcl-linter"), badCfg)

	a.validateRecursive = true
	err := a.runValidateConfig(nil, []string{root})
	if err == nil {
		t.Fatal("expected error for bad configs")
	}
	if !strings.Contains(err.Error(), "2 directory") {
		t.Errorf("expected error to mention 2 directories, got: %v", err)
	}
}

func TestValidateConfigRecursive_NoDirsFound(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	a.validateRecursive = true
	err := a.runValidateConfig(nil, []string{root})
	if err == nil {
		t.Fatal("expected error when no .hcl-linter/ dirs exist")
	}
	if !strings.Contains(err.Error(), "no .hcl-linter") {
		t.Errorf("expected 'no .hcl-linter' in error, got: %v", err)
	}
}

func TestFindHCLLinterDirs_SkipsHidden(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, ".hcl-linter"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "svc", ".hcl-linter"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A hidden sibling dir that happens to also contain one: should be skipped
	// entirely because the walk refuses to descend into hidden dirs other
	// than .hcl-linter/ itself.
	if err := os.MkdirAll(filepath.Join(root, ".cache", ".hcl-linter"), 0o755); err != nil {
		t.Fatal(err)
	}

	dirs := findHCLLinterDirs(root)
	if len(dirs) != 2 {
		t.Errorf("expected 2 dirs, got %d: %v", len(dirs), dirs)
	}
	for _, d := range dirs {
		if strings.Contains(d, ".cache") {
			t.Errorf("hidden dir should be skipped: %s", d)
		}
	}
}
