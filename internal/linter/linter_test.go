package linter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
)

func createTestConfigDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	return tmpDir
}

func setupTestConfig(t *testing.T, tmpDir string, name string, content string) {
	t.Helper()
	configDir := filepath.Join(tmpDir, ".linter-rules")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestLoader(t *testing.T, tmpDir string) *config.Loader {
	t.Helper()
	return config.NewLoader(filepath.Join(tmpDir, ".linter-rules"))
}

