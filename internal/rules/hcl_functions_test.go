package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestFindInParentFoldersRule(t *testing.T) {
	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	childDir := filepath.Join(parentDir, "child")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}

	terragruntInParent := filepath.Join(parentDir, "terragrunt.hcl")
	if err := os.WriteFile(terragruntInParent, []byte(`terraform {}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		HCLFunctions: &config.HCLFunctionsConfig{
			Enabled:                   true,
			FindInParentFoldersExists: true,
		},
	}

	tests := []struct {
		name          string
		testFile      string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:     "find_in_parent_folders with file in parent",
			testFile: filepath.Join(childDir, "terragrunt.hcl"),
			content: `include "root" {
  path = find_in_parent_folders()
}
`,
			expectIssue: false,
		},
		{
			name:     "find_in_parent_folders with custom file in parent",
			testFile: filepath.Join(childDir, "terragrunt.hcl"),
			content: `inputs = {
  config = find_in_parent_folders("terragrunt.hcl")
}
`,
			expectIssue: false,
		},
		{
			name:          "find_in_parent_folders with missing file",
			testFile:      filepath.Join(tmpDir, "terragrunt.hcl"),
			content:       `include "root" { path = find_in_parent_folders("missing.hcl") }` + "\n",
			expectIssue:   true,
			issueContains: "could not find file",
		},
	}

	r := rules.HCLFunctionsRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFile(t, tt.testFile, tt.content)
			ctx := buildContextFromFile(t, tt.testFile, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "find_in_parent_folders_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected find_in_parent_folders_exists issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected find_in_parent_folders_exists issue: %v", issues)
			}
		})
	}
}

func TestGetEnvHasDefaultRule(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		HCLFunctions: &config.HCLFunctionsConfig{
			Enabled:          true,
			GetEnvHasDefault: true,
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name:        "get_env with default value",
			content:     "locals { env = get_env(\"ENV\", \"dev\") }\n",
			expectIssue: false,
		},
		{
			name:        "get_env with extra args",
			content:     "locals { env = get_env(\"ENV\", \"dev\", \"extra\") }\n",
			expectIssue: false,
		},
		{
			name:        "get_env without default value",
			content:     "locals { env = get_env(\"ENV\") }\n",
			expectIssue: true,
		},
		{
			name:        "multiple get_env mixed",
			content:     "locals {\n  env    = get_env(\"ENV\", \"dev\")\n  region = get_env(\"REGION\")\n}\n",
			expectIssue: true,
		},
	}

	r := rules.HCLFunctionsRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "get_env_has_default" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected get_env_has_default issue, got %v", issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected get_env_has_default issue: %v", issues)
			}
		})
	}
}

func TestFindInParentFoldersInTerraformBlock(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		HCLFunctions: &config.HCLFunctionsConfig{
			Enabled:                   true,
			FindInParentFoldersExists: true,
		},
	}

	content := "terraform {\n  source = find_in_parent_folders()\n}\n"
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.HCLFunctionsRule{}
	ctx := buildContextFromFile(t, file, cfg)
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "find_in_parent_folders_exists" {
			t.Errorf("unexpected find_in_parent_folders_exists issue in terraform block: %v", issue.Message)
		}
	}
}

func TestGetEnvInInputsBlock(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		HCLFunctions: &config.HCLFunctionsConfig{
			Enabled:          true,
			GetEnvHasDefault: true,
		},
	}

	content := `inputs = {
  env     = get_env("ENV", "dev")
  region  = get_env("REGION")
  timeout = 30
}
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.HCLFunctionsRule{}
	ctx := buildContextFromFile(t, file, cfg)

	count := 0
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "get_env_has_default" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 get_env_has_default issue for REGION, got %d", count)
	}
}

func TestFunctionNotEvaluatedWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		HCLFunctions: &config.HCLFunctionsConfig{
			Enabled:                   true,
			FindInParentFoldersExists: false,
			GetEnvHasDefault:          false,
		},
	}

	content := `include "root" {
  path = find_in_parent_folders("non-existent.hcl")
}

locals {
  env = get_env("MISSING_DEFAULT")
}
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.HCLFunctionsRule{}
	ctx := buildContextFromFile(t, file, cfg)
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "find_in_parent_folders_exists" || issue.Rule == "get_env_has_default" {
			t.Errorf("unexpected issue when sub-rule is disabled: %v", issue.Rule)
		}
	}
}
