package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunLint_Verbose(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

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
	flagConfigSrc = filepath.Join(root, ".hcl-linter")
	flagVerbose = true

	if err := runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint --verbose returned error: %v", err)
	}
}

func TestRunFix_WithChanges(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		"locals {\n\n\n  x = 1\n}\n",
		`rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")
	flagVerbose = true

	if err := runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix returned error: %v", err)
	}
}

func TestRunFix_FormatModeVerbose(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := t.TempDir()
	file1 := filepath.Join(root, "a.hcl")
	file2 := filepath.Join(root, "b.hcl")
	for _, f := range []string{file1, file2} {
		if err := os.WriteFile(f, []byte("locals {\n\n\n  x = 1\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	flagFormat = true
	flagVerbose = true

	if err := runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix --format --verbose returned error: %v", err)
	}
}

func TestRunCheck_WithIssuesButOnlyWarnings(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { x = get_env("ENV") }`+"\n",
		`rules {
  hcl_functions {
    enabled             = true
    get_env_has_default = true
  }
}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")

	// Warning-only should not exit.
	if err := runCheck(nil, []string{root}); err != nil {
		t.Errorf("runCheck should not error on warnings-only: %v", err)
	}
}

func TestFilterFiles_NoMatch(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	flagFilter = []string{"other.hcl"}
	got := filterFiles([]string{"/tmp/terragrunt.hcl", "/tmp/root.hcl"})
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestMatchGlob_Combinations(t *testing.T) {
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
		// Middle glob: `a*c` => HasPrefix && HasSuffix
		{"abc", "a*c", true},
		{"xbc", "a*c", false},
		// No glob equality fallback
		{"foo", "foo", true},
		{"foo", "bar", false},
	}
	for _, tt := range tests {
		if got := matchGlob(tt.filename, tt.pattern); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.filename, tt.pattern, got, tt.want)
		}
	}
}

func TestWarnConfigIssues_EmptyDir(t *testing.T) {
	silenceStdout(t)
	// Empty configDir string returns early without stat errors.
	warnConfigIssues("")
}

func TestWarnConfigIssues_RealIssues(t *testing.T) {
	silenceStdout(t)

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "default.hcl"), []byte(`rules {
  unknown_rule_block { enabled = true }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not panic or error; prints warnings to stderr.
	warnConfigIssues(tmp)
}

func TestRunFix_FileWithParseError(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

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
	// to hit runFixMode's error branch.
	if err := os.WriteFile(filepath.Join(root, "terragrunt.hcl"), []byte("locals {\n\n  x = 1\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken.hcl"), []byte("locals { = invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	flagConfigSrc = configDir

	// Even with a broken file, runFix should return nil (errors are printed
	// per-file, not returned).
	if err := runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix returned error: %v", err)
	}
}

func TestRunFormatMode_FileWithParseError(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.hcl"), []byte("locals { = invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	flagFormat = true

	if err := runFix(nil, []string{root}); err != nil {
		t.Errorf("runFix --format returned error: %v", err)
	}
}

func TestRun_FilterWithNoMatches(t *testing.T) {
	silenceStdout(t)
	resetFlags(t)

	root := setupProject(t,
		`locals { x = 1 }`+"\n",
		`rules {}`,
	)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")
	flagFilter = []string{"definitely-not-matched.hcl"}

	// Filter eliminates all files → no files to lint but still returns nil.
	if err := runLint(nil, []string{root}); err != nil {
		t.Errorf("runLint with non-matching filter returned error: %v", err)
	}
}
