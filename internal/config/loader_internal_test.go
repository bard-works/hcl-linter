package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadConfigFile returns "unsupported config format" for non-.hcl paths so
// the caller knows to fall through to default.hcl. Cover that branch.
func TestLoadConfigFile_NonHCL(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "terragrunt")
	if err := os.WriteFile(path, []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := &Loader{}
	_, err := l.loadConfigFile(path)
	if err == nil {
		t.Fatal("expected error for non-.hcl file")
	}
}

// HasSpecificConfigForFile returns false when the loader's configDir is
// empty. Pin that branch so a zero-value Loader is safe.
func TestHasSpecificConfigForFile_EmptyConfigDir(t *testing.T) {
	l := &Loader{}
	if l.HasSpecificConfigForFile("terragrunt.hcl") {
		t.Error("expected false for empty configDir")
	}
}

// Nil-receiver branch on HasSpecificConfigForFile.
func TestHasSpecificConfigForFile_NilLoader(t *testing.T) {
	var l *Loader
	if l.HasSpecificConfigForFile("terragrunt.hcl") {
		t.Error("expected false for nil loader")
	}
}
