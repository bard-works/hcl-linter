package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestDependencyOutputsRule(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	outputsTf := `output "vpc_id" {
  type        = string
  description = "VPC ID"
  value       = "vpc-123"
}
`
	if err := os.WriteFile(filepath.Join(vpcDir, "outputs.tf"), []byte(outputsTf), 0o644); err != nil {
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
			t.Logf("Got issue: %s", issue.Message)
		}
	}
}

func TestDependencyOutputsMissingPath(t *testing.T) {
	tmpDir := t.TempDir()

	hclContent := `dependency "nonexistent" {
  config_path = "nonexistent"
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

	hasIssue := false
	for _, issue := range issues {
		if issue.Rule == "dependency_outputs" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected dependency_outputs issue for missing path, got none")
	}
}
