package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// silenceStdout redirects os.Stdout for the duration of the test so CLI
// runners don't pollute `go test -v` output.
func silenceStdout(t *testing.T) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, r)
		close(done)
	}()
	t.Cleanup(func() {
		_ = w.Close()
		<-done
		os.Stdout = orig
	})
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

// resetFlags restores global CLI flags after a test.
func resetFlags(t *testing.T) {
	t.Helper()
	flagVerbose = false
	flagConfigSrc = ""
	flagFilter = nil
	flagConcurrency = 0
	flagFormat = false
	flagDryRun = false
	flagInitForce = false
	t.Cleanup(func() {
		flagVerbose = false
		flagConfigSrc = ""
		flagFilter = nil
		flagConcurrency = 0
		flagFormat = false
		flagDryRun = false
	})
}

func TestRunLint_CleanFile(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")

	if err := runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint returned error on clean file: %v", err)
	}
}

func TestRunCheck_CleanFile(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")

	if err := runCheck(nil, []string{root}); err != nil {
		t.Errorf("runCheck returned error: %v", err)
	}
}

func TestRunFix_CleanFile(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")

	if err := runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix returned error: %v", err)
	}
}

func TestRunFix_FormatMode(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		"locals {\n\n\n  foo = \"bar\"\n}\n",
		`rules {}`,
	)
	flagFormat = true
	flagConfigSrc = filepath.Join(root, ".hcl-linter")

	if err := runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix --format returned error: %v", err)
	}
}

func TestRun_PathError(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := t.TempDir()
	flagConfigSrc = root

	missing := filepath.Join(root, "does-not-exist")
	err := runLint(nil, []string{missing})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestRun_SingleFile(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")

	target := filepath.Join(root, "terragrunt.hcl")
	if err := runLint(nil, []string{target}); err != nil {
		t.Errorf("runLint on single file: %v", err)
	}
}

func TestRun_WithFilter(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { foo = "bar" }`+"\n",
		`rules {}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")
	flagFilter = []string{"terragrunt.hcl"}

	if err := runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint with filter: %v", err)
	}
}

func TestRunValidateConfig_OK(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

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

	if err := runValidateConfig(nil, []string{root}); err != nil {
		t.Errorf("runValidateConfig: %v", err)
	}
}

func TestRunValidateConfig_BadConfig(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

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

	err := runValidateConfig(nil, []string{root})
	if err == nil {
		t.Error("expected error for unknown rule block")
	}
	if err != nil && !strings.Contains(err.Error(), "config issue") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunValidateConfig_NoConfig(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	t.Setenv("HCL_LINTER_CONFIG_DIR", "")
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())

	err := runValidateConfig(nil, nil)
	if err == nil {
		t.Skip("a .hcl-linter dir exists next to the go test binary; skipping")
	}
}

func TestFindHCLFiles_Walks(t *testing.T) {
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

	files := findHCLFiles(root)
	if len(files) != 2 {
		t.Errorf("expected 2 HCL/TF files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(f, ".git") {
			t.Errorf("hidden dir not skipped: %s", f)
		}
	}
}

func TestGetLoader(t *testing.T) {
	resetFlags(t)

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	flagConfigSrc = configDir

	loader, result := getLoader()
	if loader == nil {
		t.Fatal("expected loader, got nil")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}
