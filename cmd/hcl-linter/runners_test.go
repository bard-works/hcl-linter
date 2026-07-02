package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/termcolor"
)

// newTestApp returns an app whose output streams are discarded. Tests that
// assert on output swap in buffers for out/errOut. Each test gets its own
// instance, so there is no global flag state to reset between tests.
func newTestApp() *app {
	return &app{out: io.Discard, errOut: io.Discard, color: termcolor.ModeAuto}
}

// setupProject builds a tmp project with a `.hcl-linter/terragrunt.hcl` config
// and a `terragrunt.hcl` file to lint. Returns the project root.
func setupProject(t *testing.T, target, ruleConfig string) string {
	t.Helper()
	root := t.TempDir()

	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "terragrunt.hcl"), []byte(ruleConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "terragrunt.hcl"), []byte(target), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunLint_CleanFile(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")

	if err := a.runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint returned error on clean file: %v", err)
	}
}

func TestRunCheck_CleanFile(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")

	if err := a.runCheck(nil, []string{root}); err != nil {
		t.Errorf("runCheck returned error: %v", err)
	}
}

func TestRunFix_CleanFile(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")

	if err := a.runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix returned error: %v", err)
	}
}

func TestRunFix_FormatMode(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		"locals {\n\n\n  foo = \"bar\"\n}\n",
		`rules {}`,
	)
	a.format = true
	a.configSrc = filepath.Join(root, ".hcl-linter")

	if err := a.runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix --format returned error: %v", err)
	}
}

func TestRun_PathError(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	a.configSrc = root

	missing := filepath.Join(root, "does-not-exist")
	err := a.runLint(nil, []string{missing})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestRun_SingleFile(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")

	target := filepath.Join(root, "terragrunt.hcl")
	if err := a.runLint(nil, []string{target}); err != nil {
		t.Errorf("runLint on single file: %v", err)
	}
}

func TestRun_WithFilter(t *testing.T) {
	a := newTestApp()

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	a.configSrc = filepath.Join(root, ".hcl-linter")
	a.filter = []string{"terragrunt.hcl"}

	if err := a.runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint with filter: %v", err)
	}
}

func TestRunValidateConfig_OK(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.hcl"), []byte(`rules {
  block_order {
    enabled = true
    order   = ["include"]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.runValidateConfig(nil, []string{root}); err != nil {
		t.Errorf("runValidateConfig: %v", err)
	}
}

func TestRunValidateConfig_BadConfig(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.hcl"), []byte(`rules {
  blokc_order {
    enabled = true
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := a.runValidateConfig(nil, []string{root})
	if err == nil {
		t.Error("expected error for unknown rule block")
	}
	if err != nil && !strings.Contains(err.Error(), "config issue") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunValidateConfig_NoConfig(t *testing.T) {
	a := newTestApp()

	t.Setenv("HCL_LINTER_CONFIG_DIR", "")
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())

	err := a.runValidateConfig(nil, nil)
	if err == nil {
		t.Skip("a .hcl-linter dir exists next to the go test binary; skipping")
	}
}

func TestFindHCLFiles_Walks(t *testing.T) {
	a := newTestApp()
	root := t.TempDir()

	for _, name := range []string{"a.hcl", "b.tf", "c.json"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hidden := filepath.Join(root, ".git")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "hidden.hcl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := a.findHCLFiles(root)
	if len(files) != 2 {
		t.Errorf("expected 2 HCL/TF files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(f, ".git") {
			t.Errorf("hidden dir not skipped: %s", f)
		}
	}
}

func TestFindHCLFiles_IncludeHidden(t *testing.T) {
	a := newTestApp()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "a.hcl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	hiddenDir := filepath.Join(root, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "hidden.hcl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.includeHidden = false
	files := a.findHCLFiles(root)
	if len(files) != 1 {
		t.Errorf("expected 1 file without --include-hidden, got %d: %v", len(files), files)
	}

	a.includeHidden = true
	files = a.findHCLFiles(root)
	if len(files) != 2 {
		t.Errorf("expected 2 files with --include-hidden, got %d: %v", len(files), files)
	}
	var foundHidden bool
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "/.hidden/") {
			foundHidden = true
		}
	}
	if !foundHidden {
		t.Errorf("hidden dir file not found in results: %v", files)
	}
}

func TestRunLint_IncludeHidden(t *testing.T) {
	a := newTestApp()
	root := t.TempDir()

	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "terragrunt.hcl"), []byte(`rules {}`), 0o644); err != nil {
		t.Fatal(err)
	}

	hiddenDir := filepath.Join(root, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(hiddenDir, "terragrunt.hcl"),
		[]byte(`locals { foo = "bar" }`+"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	a.configSrc = configDir

	a.includeHidden = false
	files := a.resolveFiles(root)
	if len(files) != 0 {
		t.Errorf("expected 0 files without --include-hidden, got %d", len(files))
	}

	a.includeHidden = true
	files = a.resolveFiles(root)
	// .hcl-linter/terragrunt.hcl and .hidden/terragrunt.hcl both discovered
	if len(files) != 2 {
		t.Errorf("expected 2 files with --include-hidden, got %d: %v", len(files), files)
	}
	var foundHidden bool
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f), "/.hidden/") {
			foundHidden = true
		}
	}
	if !foundHidden {
		t.Errorf("hidden dir file not found: %v", files)
	}
}

func TestGetLoader(t *testing.T) {
	a := newTestApp()

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	a.configSrc = configDir

	loader, result := a.getLoader()
	if loader == nil {
		t.Fatal("expected loader, got nil")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestFindHCLFiles_DotRoot is a regression test: walking "." must not trip
// the hidden-directory skip on the root entry itself (whose name is "."),
// which previously made `lint .` silently process zero files.
func TestFindHCLFiles_DotRoot(t *testing.T) {
	a := newTestApp()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.hcl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	if files := a.findHCLFiles("."); len(files) != 1 {
		t.Errorf(`findHCLFiles(".") = %v, want exactly 1 file`, files)
	}
}

// TestFindHCLFiles_ExplicitHiddenRoot: a hidden directory named as the walk
// target must be walked even without --include-hidden.
func TestFindHCLFiles_ExplicitHiddenRoot(t *testing.T) {
	a := newTestApp()
	parent := t.TempDir()
	hidden := filepath.Join(parent, ".proj")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "a.hcl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if files := a.findHCLFiles(hidden); len(files) != 1 {
		t.Errorf("findHCLFiles(hidden root) = %v, want exactly 1 file", files)
	}
}
