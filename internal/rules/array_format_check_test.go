package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

// ArrayFormatRule.Check walks top-level + nested blocks and tuple
// expressions, with a special case for tuples of string literals.
// This test hits the "tuple contains non-strings" branch and the
// ObjectConsExpr recursion branch.
func TestArrayFormatCheck_TupleOfNonStrings(t *testing.T) {
	tmpDir := t.TempDir()
	content := `locals {
  numbers = [1, 2, 3]
  refs    = [dependency.foo.outputs.a, dependency.foo.outputs.b]
}
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		ArrayFormat: &config.ArrayFormatConfig{Enabled: true, MultilineThreshold: 2},
	}

	r := rules.ArrayFormatRule{}
	ctx := buildContextFromFile(t, file, cfg)

	// Non-string tuples should NOT emit array_format warnings - only
	// string-literal tuples trigger.
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "array_format" {
			t.Errorf("unexpected array_format issue for non-string tuple: %v", issue.Message)
		}
	}
}

// ArrayFormatRule.Check walks block bodies and descends into object
// attributes (ObjectConsExpr). Exercise the tuple/object/recursive paths.
func TestArrayFormatCheck_ObjectAndTupleBranches(t *testing.T) {
	tmpDir := t.TempDir()
	content := `inputs = {
  tags = ["a", "b", "c"]
  nested = {
    regions = ["eu-west-1", "us-east-1"]
  }
  mixed = [1, 2]
}

locals {
  items = ["x", "y"]
}
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		ArrayFormat: &config.ArrayFormatConfig{Enabled: true, MultilineThreshold: 2},
	}

	r := rules.ArrayFormatRule{}
	ctx := buildContextFromFile(t, file, cfg)
	issues := r.Check(ctx)

	// We don't assert exact issue count - this test primarily exists to
	// execute both tuple-of-strings and object-recursion branches.
	if issues == nil {
		t.Log("no issues emitted (acceptable; only checking traversal coverage)")
	}
}
