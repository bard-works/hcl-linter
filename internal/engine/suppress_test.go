package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/engine"
)

func TestInlineSuppression(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".hcl-linter")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `rules {
  block_order {
    enabled = true
    order   = ["include", "terraform"]
  }
  duplicates {
    enabled = true
    blocks  = ["include"]
  }
}
`
	if err := os.WriteFile(filepath.Join(cfgDir, "default.hcl"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "terragrunt.hcl")
	src := `# hcl-linter:ignore-file block_order -- generated file, order is fixed upstream
terraform {}
include "a" {}
# hcl-linter:ignore duplicates
include "a" {}
include "a" {}
`
	if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := engine.New(config.NewLoader(cfgDir))
	res, err := eng.LintFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) != 1 || res.Issues[0].Rule != "duplicates" || res.Issues[0].Location.Start.Line != 6 {
		t.Fatalf("expected only the unsuppressed duplicate on line 6, got %+v", res.Issues)
	}

	eng.DryRun = true
	fixed, err := eng.FixFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if fixed.Changes != 0 {
		t.Errorf("block_order fixer ran despite ignore-file:\n%s", fixed.Content)
	}
}
