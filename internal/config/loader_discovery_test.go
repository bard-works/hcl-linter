package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCwdConfigDir(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := getCwdConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got) != ".hcl-linter" {
		t.Errorf("expected path ending in .hcl-linter, got %s", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %s", got)
	}
}

func TestGetHomeConfigDir(t *testing.T) {
	got, err := getHomeConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got) != ".hcl-linter" {
		t.Errorf("expected path ending in .hcl-linter, got %s", got)
	}
}

func TestGetProjectConfigDir(t *testing.T) {
	got, err := getProjectConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got) != ".hcl-linter" {
		t.Errorf("expected path ending in .hcl-linter, got %s", got)
	}
}

func TestFindConfigDir_EnvSource(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HCL_LINTER_CONFIG_DIR", tmpDir)

	neutralDir := t.TempDir()
	t.Chdir(neutralDir)

	loader := &Loader{}
	result := findConfigDir(loader)
	if result.Source != ConfigSourceEnv {
		t.Errorf("got source %v, want ConfigSourceEnv", result.Source)
	}
	if result.SourcePath != configDir {
		t.Errorf("got path %s, want %s", result.SourcePath, configDir)
	}
}

func TestFindConfigDir_CwdSource(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HCL_LINTER_CONFIG_DIR", "")
	t.Chdir(tmpDir)

	result := findConfigDir(&Loader{})
	if result.Source != ConfigSourceCwd {
		t.Errorf("got source %v, want ConfigSourceCwd", result.Source)
	}
}

func TestFindConfigDir_None(t *testing.T) {
	t.Setenv("HCL_LINTER_CONFIG_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	result := findConfigDir(&Loader{})
	if result.Source != ConfigSourceNone && result.Source != ConfigSourceProject {
		t.Errorf("got source %v, want ConfigSourceNone or ConfigSourceProject (project binary dir may have .hcl-linter)", result.Source)
	}
}

func TestGetMaxConcurrency_Default(t *testing.T) {
	t.Setenv("HCL_LINTER_MAX_CONCURRENCY", "")
	got := GetMaxConcurrency(nil)
	if got <= 0 {
		t.Errorf("expected >0, got %d", got)
	}
}

func TestGetMaxConcurrency_FromEnv(t *testing.T) {
	t.Setenv("HCL_LINTER_MAX_CONCURRENCY", "4")
	if got := GetMaxConcurrency(nil); got != 4 {
		t.Errorf("got %d, want 4", got)
	}
}

func TestGetMaxConcurrency_FromConfig(t *testing.T) {
	t.Setenv("HCL_LINTER_MAX_CONCURRENCY", "")
	got := GetMaxConcurrency(&Rules{MaxConcurrency: 7})
	if got != 7 {
		t.Errorf("got %d, want 7", got)
	}
}

func TestGetMaxConcurrency_EnvOverridesConfig(t *testing.T) {
	t.Setenv("HCL_LINTER_MAX_CONCURRENCY", "2")
	got := GetMaxConcurrency(&Rules{MaxConcurrency: 9})
	if got != 2 {
		t.Errorf("env should win: got %d, want 2", got)
	}
}

func TestGetMaxConcurrency_InvalidEnvFallsBack(t *testing.T) {
	t.Setenv("HCL_LINTER_MAX_CONCURRENCY", "not-a-number")
	got := GetMaxConcurrency(&Rules{MaxConcurrency: 3})
	if got != 3 {
		t.Errorf("should fall back to config: got %d, want 3", got)
	}
}
