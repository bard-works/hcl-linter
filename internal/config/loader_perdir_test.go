package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const rootOrderConfig = `rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}`

const nestedOrderConfig = `rules {
  block_order {
    enabled = true
    order   = ["locals", "terraform"]
  }
}`

// writeConfigFile is a small helper for the per-dir tests. Tests in this
// package already use direct os.WriteFile, so this trims boilerplate only
// for the repeated ".hcl-linter/<name>.hcl" pattern.
func writeConfigFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPerDir_CloserWins verifies that when a nested .hcl-linter/ dir exists
// between the file and the root config, the nested config is used.
func TestPerDir_CloserWins(t *testing.T) {
	root := t.TempDir()

	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "terragrunt.hcl", rootOrderConfig)

	svcDir := filepath.Join(root, "svc")
	svcConfig := filepath.Join(svcDir, ".hcl-linter")
	writeConfigFile(t, svcConfig, "terragrunt.hcl", nestedOrderConfig)

	target := filepath.Join(svcDir, "terragrunt.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)
	rules, err := loader.LoadForFile(target)
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	if rules.BlockOrder == nil {
		t.Fatal("BlockOrder nil")
	}
	got := rules.BlockOrder.Order
	want := []string{"locals", "terraform"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("expected nested config order %v, got %v", want, got)
	}
}

// TestPerDir_EmptyNestedDirFallsThrough verifies that a nested `.hcl-linter/`
// without a matching per-name file AND without default.hcl is skipped, and
// the walk continues upward to a dir that does have a match.
func TestPerDir_EmptyNestedDirFallsThrough(t *testing.T) {
	root := t.TempDir()

	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "terragrunt.hcl", rootOrderConfig)

	// Nested dir has a `.hcl-linter/` but only an unrelated config.
	svcDir := filepath.Join(root, "svc")
	writeConfigFile(t, filepath.Join(svcDir, ".hcl-linter"), "root.hcl", nestedOrderConfig)

	target := filepath.Join(svcDir, "terragrunt.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)
	rules, err := loader.LoadForFile(target)
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	if rules.BlockOrder == nil {
		t.Fatal("BlockOrder nil")
	}
	if len(rules.BlockOrder.Order) != 3 {
		t.Errorf("expected fallback to root config (3 items), got %v", rules.BlockOrder.Order)
	}
}

// TestPerDir_NestedDefaultHclMatches verifies that a nested `.hcl-linter/`
// with only a default.hcl (no per-name file) still wins for any basename.
func TestPerDir_NestedDefaultHclMatches(t *testing.T) {
	root := t.TempDir()

	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "default.hcl", rootOrderConfig)

	svcDir := filepath.Join(root, "svc")
	writeConfigFile(t, filepath.Join(svcDir, ".hcl-linter"), "default.hcl", nestedOrderConfig)

	target := filepath.Join(svcDir, "some-other-name.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)
	rules, err := loader.LoadForFile(target)
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	if rules.BlockOrder == nil || len(rules.BlockOrder.Order) != 2 {
		t.Errorf("expected nested default.hcl to win, got %+v", rules.BlockOrder)
	}
}

// TestPerDir_ExtendsResolvesInNestedDir verifies that `extends = "default"`
// inside a nested `.hcl-linter/` resolves against files in the *same* nested
// dir - not the root config dir.
func TestPerDir_ExtendsResolvesInNestedDir(t *testing.T) {
	root := t.TempDir()

	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "default.hcl", rootOrderConfig)

	svcDir := filepath.Join(root, "svc")
	nestedConfig := filepath.Join(svcDir, ".hcl-linter")
	writeConfigFile(t, nestedConfig, "default.hcl", `rules {
  block_order {
    enabled = true
    order   = ["dependency", "inputs"]
  }
  duplicates {
    enabled = true
    blocks  = ["locals"]
  }
}`)
	writeConfigFile(t, nestedConfig, "terragrunt.hcl", `extends = "default"

rules {
  block_order {
    enabled = true
    order   = ["locals"]
  }
}`)

	target := filepath.Join(svcDir, "terragrunt.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)
	rules, err := loader.LoadForFile(target)
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	if rules.BlockOrder == nil || len(rules.BlockOrder.Order) != 1 || rules.BlockOrder.Order[0] != "locals" {
		t.Errorf("child block_order did not override base: %+v", rules.BlockOrder)
	}
	if rules.Duplicates == nil || len(rules.Duplicates.Blocks) != 1 || rules.Duplicates.Blocks[0] != "locals" {
		t.Errorf("expected duplicates inherited from nested default, got %+v", rules.Duplicates)
	}
}

// TestPerDir_FileOutsideRootSkipsWalk verifies that a file living outside
// the root configDir's parent tree does not trigger an upward walk and
// uses the root configDir directly.
func TestPerDir_FileOutsideRootSkipsWalk(t *testing.T) {
	rootParent := t.TempDir()
	rootConfig := filepath.Join(rootParent, ".hcl-linter")
	writeConfigFile(t, rootConfig, "terragrunt.hcl", rootOrderConfig)

	// Separate temp dir with its own .hcl-linter/ that must be ignored.
	outside := t.TempDir()
	writeConfigFile(t, filepath.Join(outside, ".hcl-linter"), "terragrunt.hcl", nestedOrderConfig)

	target := filepath.Join(outside, "terragrunt.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)
	rules, err := loader.LoadForFile(target)
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	if rules.BlockOrder == nil || len(rules.BlockOrder.Order) != 3 {
		t.Errorf("expected root (3-item) config when file is outside root tree, got %+v", rules.BlockOrder)
	}
}

// TestPerDir_HasSpecificConfigForFile verifies discovery helpers see the
// nested config, not just the root one.
func TestPerDir_HasSpecificConfigForFile(t *testing.T) {
	root := t.TempDir()

	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "default.hcl", rootOrderConfig)

	svcDir := filepath.Join(root, "svc")
	writeConfigFile(t, filepath.Join(svcDir, ".hcl-linter"), "terragrunt.hcl", nestedOrderConfig)

	loader := NewLoader(rootConfig)

	nestedTarget := filepath.Join(svcDir, "terragrunt.hcl")
	if !loader.HasSpecificConfigForFile(nestedTarget) {
		t.Error("expected HasSpecificConfigForFile to find nested terragrunt.hcl")
	}

	// A different basename under the nested dir: no nested per-name match, but
	// a nested default exists? no. Falls back to root default.
	otherTarget := filepath.Join(svcDir, "other.hcl")
	if loader.HasSpecificConfigForFile(otherTarget) {
		t.Error("expected HasSpecificConfigForFile to return false when no per-name match")
	}
	if !loader.HasConfigForFile(otherTarget) {
		t.Error("expected HasConfigForFile to return true via root default.hcl fallback")
	}
}

// TestPerDir_ResolveCacheIsConcurrencySafe exercises the cache under
// concurrent LoadForFile calls - the engine uses LintFiles with worker
// goroutines, so the cache must be safe.
func TestPerDir_ResolveCacheIsConcurrencySafe(t *testing.T) {
	root := t.TempDir()
	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "terragrunt.hcl", rootOrderConfig)

	svcDir := filepath.Join(root, "svc")
	writeConfigFile(t, filepath.Join(svcDir, ".hcl-linter"), "terragrunt.hcl", nestedOrderConfig)

	target := filepath.Join(svcDir, "terragrunt.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := loader.LoadForFile(target); err != nil {
				t.Errorf("LoadForFile: %v", err)
			}
		}()
	}
	wg.Wait()
}

// TestPerDir_NestedMustBeActualHclLinter guards against accidentally
// matching a dir literally named ".hcl-linter" somewhere unexpected - the
// walk must test for dir with a usable config file, not just presence.
func TestPerDir_NestedMustBeActualHclLinter(t *testing.T) {
	root := t.TempDir()
	rootConfig := filepath.Join(root, ".hcl-linter")
	writeConfigFile(t, rootConfig, "terragrunt.hcl", rootOrderConfig)

	svcDir := filepath.Join(root, "svc")
	emptyNested := filepath.Join(svcDir, ".hcl-linter")
	if err := os.MkdirAll(emptyNested, 0o755); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(svcDir, "terragrunt.hcl")
	if err := os.WriteFile(target, []byte("locals {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(rootConfig)
	rules, err := loader.LoadForFile(target)
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	if rules.BlockOrder == nil || len(rules.BlockOrder.Order) != 3 {
		t.Errorf("empty nested .hcl-linter/ should be ignored; got %+v", rules.BlockOrder)
	}
	// Extra sanity: the walk helper itself returns the root.
	if got := loader.resolveConfigDirForFile(target); !strings.EqualFold(got, rootConfig) {
		t.Errorf("resolveConfigDirForFile: got %s, want %s", got, rootConfig)
	}
}
