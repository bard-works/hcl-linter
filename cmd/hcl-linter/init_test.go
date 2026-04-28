package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

func captureInitOut(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := initOut
	initOut = buf
	t.Cleanup(func() { initOut = prev })
	return buf
}

func initTestProject(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("locals { foo = \"bar\" }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInit_WritesDefaultAndPerFilename(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := initTestProject(t,
		"terragrunt.hcl",
		"svc/terragrunt.hcl",
		"svc/root.hcl",
	)
	captureInitOut(t)

	if err := runInit(nil, []string{root}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configDir := filepath.Join(root, ".hcl-linter")
	for _, name := range []string{"default.hcl", "terragrunt.hcl", "root.hcl"} {
		if _, err := os.Stat(filepath.Join(configDir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	def, err := os.ReadFile(filepath.Join(configDir, "default.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(def), "block_order") {
		t.Errorf("default.hcl missing block_order:\n%s", def)
	}

	over, err := os.ReadFile(filepath.Join(configDir, "terragrunt.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(over), `extends = "default"`) {
		t.Errorf("per-filename config missing extends:\n%s", over)
	}
}

func TestInit_DryRunDoesNotWrite(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := initTestProject(t, "terragrunt.hcl")
	flagDryRun = true
	buf := captureInitOut(t)

	if err := runInit(nil, []string{root}); err != nil {
		t.Fatalf("runInit --dry-run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".hcl-linter")); !os.IsNotExist(err) {
		t.Error(".hcl-linter/ should not exist after dry-run")
	}

	out := buf.String()
	if !strings.Contains(out, "default.hcl") {
		t.Errorf("dry-run output missing default.hcl:\n%s", out)
	}
	if !strings.Contains(out, "block_order") {
		t.Errorf("dry-run should print file contents:\n%s", out)
	}
}

func TestInit_RefusesNonEmptyDir(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := initTestProject(t, "terragrunt.hcl")
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(configDir, "existing.hcl")
	if err := os.WriteFile(existing, []byte("# kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := captureInitOut(t)

	err := runInit(nil, []string{root})
	if err == nil {
		t.Fatal("expected error for non-empty .hcl-linter/")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force: %v", err)
	}

	kept, readErr := os.ReadFile(existing)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(kept) != "# kept\n" {
		t.Error("existing file was modified despite refusal")
	}

	out := buf.String()
	if !strings.Contains(out, "existing.hcl") {
		t.Errorf("expected listing of existing contents, got:\n%s", out)
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := initTestProject(t, "terragrunt.hcl")
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(configDir, "default.hcl")
	if err := os.WriteFile(stale, []byte("# stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flagInitForce = true
	captureInitOut(t)

	if err := runInit(nil, []string{root}); err != nil {
		t.Fatalf("runInit --force: %v", err)
	}

	got, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "block_order") {
		t.Errorf("--force did not overwrite default.hcl:\n%s", got)
	}
}

func TestInit_NoHCLFiles(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := t.TempDir()
	captureInitOut(t)

	if err := runInit(nil, []string{root}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, ".hcl-linter"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "default.hcl" {
		t.Errorf("expected only default.hcl, got %v", entries)
	}
}

func TestInit_OutputPassesValidateConfig(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := initTestProject(t, "terragrunt.hcl", "root.hcl", "svc/service.hcl")
	captureInitOut(t)

	if err := runInit(nil, []string{root}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	issues := config.ValidateDir(filepath.Join(root, ".hcl-linter"))
	if len(issues) != 0 {
		t.Errorf("validate-config on generated layout should be clean, got %d issue(s):", len(issues))
		for _, i := range issues {
			t.Errorf("  %s", i)
		}
	}
}

func TestInit_SkipsDefaultHCLSource(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := initTestProject(t, "default.hcl", "terragrunt.hcl")
	captureInitOut(t)

	if err := runInit(nil, []string{root}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	def, err := os.ReadFile(filepath.Join(root, ".hcl-linter", "default.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(def), "block_order") {
		t.Errorf("default.hcl must be the baseline template, not an extends stub:\n%s", def)
	}
}

func TestUniqueBasenames(t *testing.T) {
	files := []string{
		"/a/terragrunt.hcl",
		"/b/terragrunt.hcl",
		"/c/root.hcl",
		"/d/service.hcl",
		"/e/root.hcl",
	}
	got := uniqueBasenames(files)
	want := []string{"root.hcl", "service.hcl", "terragrunt.hcl"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWriteInitFilesError(t *testing.T) {
	tmp := t.TempDir()
	// Create a file where the directory should go - MkdirAll will fail
	blocker := filepath.Join(tmp, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeInitFiles(filepath.Join(blocker, "subdir"), map[string]string{})
	if err == nil {
		t.Error("expected error when configDir can't be created")
	}
}

func TestWriteInitFilesSymlinkRejected(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	realFile := filepath.Join(tmp, "real.hcl")
	if err := os.WriteFile(realFile, []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(configDir, "link.hcl")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	err := writeInitFiles(configDir, map[string]string{symlink: "rules {}"})
	if err == nil {
		t.Fatal("expected error for symlink write")
	}
}
