package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func buildContextFromFile(t *testing.T, filePath string, cfg *config.Rules) *rules.Context {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	p := hclparse.NewParser()
	file, diags := p.ParseHCL(content, filePath)
	if diags.HasErrors() {
		t.Fatalf("parse error: %s", diags.Error())
	}
	return &rules.Context{
		FilePath: filePath,
		Content:  content,
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   cfg,
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyPathExistsRule(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing-module")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, DependencyPathExists: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:        "dependency with existing path",
			content:     `dependency "vpc" { config_path = "existing-module" }` + "\n",
			expectIssue: false,
		},
		{
			name:          "dependency with non-existing path",
			content:       `dependency "vpc" { config_path = "non-existent-module" }` + "\n",
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name:          "dependency with parent path",
			content:       `dependency "vpc" { config_path = "../vpc" }` + "\n",
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name: "multiple dependencies - one missing",
			content: `dependency "vpc" { config_path = "existing-module" }
dependency "db" { config_path = "missing-db" }
`,
			expectIssue:   true,
			issueContains: "missing-db",
		},
	}

	r := rules.TerragruntRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "dependency_path_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected dependency_path_exists issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected dependency_path_exists issue: %v", issues)
			}
		})
	}
}

func TestDependencyPathExistsWithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, DependencyPathExists: true},
	}

	content := fmt.Sprintf(`dependency "vpc" { config_path = "%s" }`+"\n", filepath.ToSlash(existingDir))
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.TerragruntRule{}
	ctx := buildContextFromFile(t, file, cfg)
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "dependency_path_exists" {
			t.Errorf("unexpected dependency_path_exists issue: %v", issue.Message)
		}
	}
}

func TestIncludePathExistsRule(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing-parent")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, IncludePathExists: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:        "include with existing path",
			content:     `include "root" { path = "existing-parent" }` + "\n",
			expectIssue: false,
		},
		{
			name:          "include with non-existing path",
			content:       `include "root" { path = "non-existent" }` + "\n",
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name:        "include with function call - skipped",
			content:     `include "root" { path = find_in_parent_folders() }` + "\n",
			expectIssue: false,
		},
	}

	r := rules.TerragruntRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "include_path_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected include_path_exists issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected include_path_exists issue: %v", issues)
			}
		})
	}
}

func TestIncludePathExistsWithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, IncludePathExists: true},
	}

	content := fmt.Sprintf(`include "root" { path = "%s" }`+"\n", filepath.ToSlash(existingDir))
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.TerragruntRule{}
	ctx := buildContextFromFile(t, file, cfg)
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "include_path_exists" {
			t.Errorf("unexpected include_path_exists issue: %v", issue.Message)
		}
	}
}

func TestRemoteStateConfigRule(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, RemoteStateConfig: true},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "remote_state with backend",
			content: `terraform {
  source = "./module"
  remote_state {
    backend = "s3"
    config { bucket = "my-bucket" }
  }
}
`,
			expectIssue: false,
		},
		{
			name: "remote_state without backend",
			content: `terraform {
  source = "./module"
  remote_state {
    config { bucket = "my-bucket" }
  }
}
`,
			expectIssue: true,
		},
		{
			name:        "terraform without remote_state",
			content:     `terraform { source = "./module" }` + "\n",
			expectIssue: false,
		},
		{
			name:        "empty remote_state block",
			content:     "terraform {\n  source = \"./module\"\n  remote_state {}\n}\n",
			expectIssue: true,
		},
	}

	r := rules.TerragruntRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "remote_state_config" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected remote_state_config issue, got %v", issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected remote_state_config issue: %v", issues)
			}
		})
	}
}

func TestRemoteStateConfigWithBackendAndEmptyString(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, RemoteStateConfig: true},
	}

	content := `terraform {
  source = "./module"
  remote_state {
    backend = ""
    config { bucket = "my-bucket" }
  }
}
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.TerragruntRule{}
	ctx := buildContextFromFile(t, file, cfg)

	hasIssue := false
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "remote_state_config" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected remote_state_config issue for empty backend string")
	}
}

func TestMultipleDependenciesWithMixedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		Terragrunt: &config.TerragruntConfig{Enabled: true, DependencyPathExists: true},
	}

	content := `dependency "vpc" { config_path = "vpc" }
dependency "db" { config_path = "non-existent-db" }
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.TerragruntRule{}
	ctx := buildContextFromFile(t, file, cfg)

	count := 0
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "dependency_path_exists" {
			count++
			if !strings.Contains(issue.Message, "non-existent-db") {
				t.Errorf("issue should mention non-existent-db, got: %v", issue.Message)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 issue for non-existent-db, got %d", count)
	}
}
