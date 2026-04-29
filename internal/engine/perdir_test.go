package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
)

// TestPerDir_NestedConfigsWinInLintFiles verifies end-to-end that when
// LintFiles processes files under different nested .hcl-linter/ dirs, each
// file is evaluated against the config closest to it.
func TestPerDir_NestedConfigsWinInLintFiles(t *testing.T) {
	root := t.TempDir()
	rootConfig := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(rootConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	// Root requires order [include, locals, terraform].
	rootCfg := `rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}`
	if err := os.WriteFile(filepath.Join(rootConfig, "terragrunt.hcl"), []byte(rootCfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nested service dir requires [locals, terraform, include] - inverse.
	svcDir := filepath.Join(root, "svc")
	svcConfig := filepath.Join(svcDir, ".hcl-linter")
	if err := os.MkdirAll(svcConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	svcCfg := `rules {
  block_order {
    enabled = true
    order   = ["locals", "terraform", "include"]
  }
}`
	if err := os.WriteFile(filepath.Join(svcConfig, "terragrunt.hcl"), []byte(svcCfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Both files share the same source order: include → locals → terraform.
	// That satisfies the root rule but violates the nested rule.
	source := `include "root" {
  path = "x"
}

locals {
  a = 1
}

terraform {
  source = "s"
}
`
	rootFile := filepath.Join(root, "terragrunt.hcl")
	svcFile := filepath.Join(svcDir, "terragrunt.hcl")
	if err := os.WriteFile(rootFile, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svcFile, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := config.NewLoader(rootConfig)
	eng := New(loader)

	results := eng.LintFiles([]string{rootFile, svcFile}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var rootIssues, svcIssues int
	for _, r := range results {
		switch r.File {
		case rootFile:
			rootIssues = len(r.Issues)
		case svcFile:
			svcIssues = len(r.Issues)
		}
	}

	if rootIssues != 0 {
		t.Errorf("root file should have no block_order issues, got %d", rootIssues)
	}
	if svcIssues == 0 {
		t.Error("svc file should report block_order violations under the nested config")
	}
}
