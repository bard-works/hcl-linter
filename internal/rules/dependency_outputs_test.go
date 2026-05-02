package rules_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDependencyOutputsNoConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// dependency block with no config_path → depGetPath returns "" → early return, no issue
	hclContent := `dependency "vpc" { mock_outputs = {} }` + "\n"
	hclFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(hclFile, []byte(hclContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyOutputs: &config.DependencyOutputsConfig{Enabled: true},
	}
	ctx := buildContextFromFile(t, hclFile, cfg)
	issues := (rules.DependencyOutputsRule{}).Check(ctx)
	for _, issue := range issues {
		if issue.Rule == "dependency_outputs" {
			t.Errorf("unexpected dependency_outputs issue for block without config_path: %s", issue.Message)
		}
	}
}

func TestDependencyOutputsMockOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mockJSON := `{"outputs":{"vpc_id":{"value":"vpc-123","type":"string"}}}`
	if err := os.WriteFile(filepath.Join(vpcDir, ".mock-outputs.json"), []byte(mockJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	hclContent := `dependency "vpc" { config_path = "vpc" }` + "\n"
	hclFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(hclFile, []byte(hclContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyOutputs: &config.DependencyOutputsConfig{Enabled: true},
	}
	ctx := buildContextFromFile(t, hclFile, cfg)
	// Should not report "outputs not found" since mock outputs exist
	for _, issue := range (rules.DependencyOutputsRule{}).Check(ctx) {
		if issue.Rule == "dependency_outputs" && strings.Contains(issue.Message, "outputs not found") {
			t.Errorf("unexpected 'outputs not found' issue when mock outputs exist: %s", issue.Message)
		}
	}
}

func TestDependencyOutputsMalformedMockOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Invalid JSON → depParseMockOutputs returns nil, outputs stay empty
	if err := os.WriteFile(filepath.Join(vpcDir, ".mock-outputs.json"), []byte("not-json{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	hclContent := `dependency "vpc" { config_path = "vpc" }` + "\n"
	hclFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(hclFile, []byte(hclContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyOutputs: &config.DependencyOutputsConfig{Enabled: true},
	}
	ctx := buildContextFromFile(t, hclFile, cfg)
	issues := (rules.DependencyOutputsRule{}).Check(ctx)
	// With no tf files and malformed mock, outputs = nil → "outputs not found" warning
	hasIssue := false
	for _, issue := range issues {
		if issue.Rule == "dependency_outputs" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected dependency_outputs issue when no valid outputs found")
	}
}

func TestDependencyOutputsCircularDetection(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outputsTf := `output "vpc_id" { type = string; value = "x" }` + "\n"
	if err := os.WriteFile(filepath.Join(vpcDir, "outputs.tf"), []byte(outputsTf), 0o644); err != nil {
		t.Fatal(err)
	}

	// Two dependency blocks pointing at the same path → second triggers circular detection
	hclContent := `dependency "vpc1" {
  config_path = "vpc"
}
dependency "vpc2" {
  config_path = "vpc"
}
`
	hclFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(hclFile, []byte(hclContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyOutputs: &config.DependencyOutputsConfig{Enabled: true},
	}
	ctx := buildContextFromFile(t, hclFile, cfg)
	issues := (rules.DependencyOutputsRule{}).Check(ctx)

	hasCircular := false
	for _, issue := range issues {
		if issue.Rule == "dependency_outputs" && strings.Contains(issue.Message, "circular") {
			hasCircular = true
		}
	}
	if !hasCircular {
		t.Error("expected circular dependency warning for two deps with same config_path")
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
