package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestDependencyOutputsWithMockOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mockJSON := `{
  "outputs": {
    "vpc_id":     {"value": "vpc-mock-123", "type": "string"},
    "cidr_block": {"value": "10.0.0.0/16",  "type": "string"}
  }
}
`
	if err := os.WriteFile(filepath.Join(vpcDir, ".mock-outputs.json"), []byte(mockJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	hclContent := `dependency "vpc" {
  config_path = "vpc"
}

inputs = {
  vpc_id = dependency.vpc.outputs.vpc_id
}
`
	hclFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(hclFile, []byte(hclContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyOutputs: &config.DependencyOutputsConfig{Enabled: true},
	}

	r := rules.DependencyOutputsRule{}
	ctx := buildContextFromFile(t, hclFile, cfg)
	issues := r.Check(ctx)

	for _, issue := range issues {
		if issue.Rule == "dependency_outputs" {
			t.Errorf("unexpected issue with mock outputs: %v", issue.Message)
		}
	}
}

func TestDependencyOutputsInvalidMockJSON(t *testing.T) {
	tmpDir := t.TempDir()
	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(vpcDir, ".mock-outputs.json"), []byte(`{"this is not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}

	hclContent := `dependency "vpc" { config_path = "vpc" }` + "\n"
	hclFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(hclFile, []byte(hclContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{DependencyOutputs: &config.DependencyOutputsConfig{Enabled: true}}
	r := rules.DependencyOutputsRule{}
	ctx := buildContextFromFile(t, hclFile, cfg)

	// Invalid mock JSON → depParseMockOutputs returns nil → rule warns that outputs not found.
	var sawNotFound bool
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "dependency_outputs" {
			sawNotFound = true
		}
	}
	if !sawNotFound {
		t.Error("expected 'outputs not found' warning when mock JSON is invalid")
	}
}
