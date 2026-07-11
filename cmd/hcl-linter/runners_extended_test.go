package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bard-works/hcl-linter/internal/engine"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

func TestRunLint_Verbose(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`include "BadName" { path = "./x" }`+"\n",
		`rules {
  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["include"]
  }
}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")
	a.verbose = true

	if err := a.runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint --verbose returned error: %v", err)
	}
}

func TestRunFix_WithChanges(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		"locals {\n\n\n  x = 1\n}\n",
		`rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")
	a.verbose = true

	if err := a.runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix returned error: %v", err)
	}
}

func TestRunFix_FormatModeVerbose(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	file1 := filepath.Join(root, "a.hcl")
	file2 := filepath.Join(root, "b.hcl")
	for _, f := range []string{file1, file2} {
		if err := os.WriteFile(f, []byte("locals {\n\n\n  x = 1\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.format = true
	a.verbose = true

	if err := a.runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix --format --verbose returned error: %v", err)
	}
}

func TestRunCheck_WithIssuesButOnlyWarnings(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { x = get_env("ENV") }`+"\n",
		`rules {
  hcl_functions {
    enabled             = true
    get_env_has_default = true
  }
}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")

	// Warning-only should not exit.
	if err := a.runCheck(nil, []string{root}); err != nil {
		t.Errorf("runCheck should not error on warnings-only: %v", err)
	}
}

func TestFilterFiles_NoMatch(t *testing.T) {
	a := newTestApp()

	a.filter = []string{"other.hcl"}
	got := a.filterFiles([]string{"/tmp/terragrunt.hcl", "/tmp/root.hcl"})
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestMatchPattern_Combinations(t *testing.T) {
	tests := []struct {
		filename string
		pattern  string
		want     bool
	}{
		{"terragrunt.hcl", "terragrunt.hcl", true},
		{"root.hcl", "terragrunt.hcl", false},
		{"a.hcl", "*.hcl", true},
		{"b.tf", "*.hcl", false},
		{"terragrunt.hcl", "terr*", true},
		{"service.hcl", "terr*", false},
		{"abc", "a*c", true},
		{"xbc", "a*c", false},
		{"foo", "foo", true},
		{"foo", "bar", false},
		// filepath.Match correctly handles multiple wildcards, unlike the
		// prior hand-rolled matchGlob (which degraded these to false).
		{"xbc", "*b*c", true},
		{"abc", "a*b*", true},
		{"a*b", "a*b*", true},
	}
	for _, tt := range tests {
		if got := matchPattern(tt.pattern, tt.filename); got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.filename, got, tt.want)
		}
	}
}

func TestWarnConfigIssues_EmptyDir(_ *testing.T) {
	a := newTestApp()
	// Empty configDir string returns early without stat errors.
	a.warnConfigIssues("")
}

func TestWarnConfigIssues_RealIssues(t *testing.T) {
	a := newTestApp()

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "default.hcl"), []byte(`rules {
  unknown_rule_block { enabled = true }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not panic or error; prints warnings to the diagnostic stream.
	a.warnConfigIssues(tmp)
}

func TestRunFix_FileWithParseError(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Use terragrunt.hcl to trigger specific-config match.
	if err := os.WriteFile(filepath.Join(configDir, "terragrunt.hcl"), []byte(`rules {
  blank_lines { enabled = true; within_blocks = true }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Valid file to ensure fix processing runs, plus one with a parse error
	// to hit runFix's error branch.
	if err := os.WriteFile(
		filepath.Join(root, "terragrunt.hcl"),
		[]byte("locals {\n\n  x = 1\n}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken.hcl"), []byte("locals { = invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.configSrc = configDir

	// Even with a broken file, runFix should return nil (errors are printed
	// per-file, not returned).
	if err := a.runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix returned error: %v", err)
	}
}

func TestRunFormatMode_FileWithParseError(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.hcl"), []byte("locals { = invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.format = true

	if err := a.runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix --format returned error: %v", err)
	}
}

func TestRun_FilterWithNoMatches(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { x = 1 }`+"\n",
		`rules {}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")
	a.filter = []string{"definitely-not-matched.hcl"}

	// Filter eliminates all files → no files to lint but still returns nil.
	if err := a.runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint with non-matching filter returned error: %v", err)
	}
}

func TestRunLint_PathError(t *testing.T) {
	a := newTestApp()

	if err := a.runLint(nil, []string{"/nonexistent/path/does/not/exist"}); err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestRunCheck_PathError(t *testing.T) {
	a := newTestApp()

	if err := a.runCheck(nil, []string{"/nonexistent/path/does/not/exist"}); err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestRunFix_PathError(t *testing.T) {
	a := newTestApp()

	if err := a.runFix(nil, []string{"/nonexistent/path/does/not/exist"}); err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestRunFormatMode_PathError(t *testing.T) {
	a := newTestApp()
	a.format = true

	if err := a.runFix(nil, []string{"/nonexistent/path/does/not/exist"}); err == nil {
		t.Error("expected error for nonexistent path in format mode, got nil")
	}
}

func TestRunLintMode_CheckModeWithErrors(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`include "BadName" { path = "./x" }`+"\n",
		`rules {
  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["include"]
  }
}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")

	// runCheck returns an error when lint errors are found (check mode),
	// and that error must map to the findings exit code (1), not exec (3).
	err := a.runCheck(nil, []string{root})
	if err == nil {
		t.Fatal("expected error from runCheck with lint errors, got nil")
	}
	var fe *findingsError
	if !errors.As(err, &fe) {
		t.Errorf("expected findingsError, got %T: %v", err, err)
	}
}

func TestRunLintMode_VerboseMultipleFiles(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.hcl"), []byte("rules {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.hcl", "b.hcl"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("locals { x = 1 }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.configSrc = configDir
	a.verbose = true

	if err := a.runLint(nil, []string{root}); err != nil {
		t.Errorf("verbose runLint returned error: %v", err)
	}
}

func TestPrintFixResultWithError(t *testing.T) {
	a := newTestApp()

	result := &engine.FixResult{
		File:  "/tmp/test.hcl",
		Error: errors.New("write failed"),
	}
	n := a.printFixResult(result)
	if n != 0 {
		t.Errorf("expected 0 changes for error result, got %d", n)
	}
}

func TestHandleFixResults_DryRunReadError(_ *testing.T) {
	a := newTestApp()

	// A result pointing to a nonexistent file - ReadFile will fail in dry-run.
	results := []*engine.FixResult{
		{File: "/nonexistent/path.hcl", Changes: 1, Content: "modified", Success: true},
	}
	_, _ = a.handleFixResults(results, true)
}

func TestPrintExplainTable_ColorEnabled(t *testing.T) {
	a := newTestApp()
	if err := termcolor.SetMode(termcolor.ModeAlways); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	a.printExplainTable()
}

func TestWriteInitFilesWriteError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based tests are not supported on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not block writes when running as root")
	}
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "cfg")
	if err := os.MkdirAll(configDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(configDir, 0o755) })

	proposed := map[string]string{
		filepath.Join(configDir, "default.hcl"): "rules {}\n",
	}
	if err := writeInitFiles(configDir, proposed); err == nil {
		t.Error("expected error writing to read-only dir, got nil")
	}
}

func TestFindHCLLinterDirs_WithFiles(t *testing.T) {
	tmp := t.TempDir()
	// Create a real .hcl-linter dir and a sibling file
	hclDir := filepath.Join(tmp, ".hcl-linter")
	if err := os.MkdirAll(hclDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Regular file in root (exercises the !d.IsDir() walkFn path)
	if err := os.WriteFile(filepath.Join(tmp, "file.hcl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs := findHCLLinterDirs(tmp)
	if len(dirs) != 1 {
		t.Errorf("expected 1 .hcl-linter dir, got %d: %v", len(dirs), dirs)
	}
}

// TestFindHCLFiles_SymlinkLoopTerminates locks in the invariant that the
// walker does not follow directory symlinks: a self-referencing symlink cycle
// must neither hang the walk nor duplicate results. If symlink-following is
// ever added, this test forces the author to handle loops explicitly.
func TestFindHCLFiles_SymlinkLoopTerminates(t *testing.T) {
	a := newTestApp()
	root := t.TempDir()

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.hcl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sub/loop -> root creates a cycle if the walker follows symlinks.
	if err := os.Symlink(root, filepath.Join(sub, "loop")); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	files := a.findHCLFiles(root)
	if len(files) != 1 {
		t.Errorf("expected exactly 1 file despite symlink loop, got %d: %v", len(files), files)
	}
}
