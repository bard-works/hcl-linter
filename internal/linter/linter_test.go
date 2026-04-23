package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
)

func createTestConfigDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	return tmpDir
}

func setupTestConfig(t *testing.T, tmpDir string, name string, content string) {
	t.Helper()
	configFile := filepath.Join(tmpDir, name)
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBlockOrderRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform"]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name        string
		content     string
		expectIssue bool
		issueRule   string
	}{
		{
			name: "correct order",
			content: `include "root" {}
locals {}
terraform {}
`,
			expectIssue: false,
		},
		{
			name: "wrong order - terraform before locals",
			content: `include "root" {}
terraform {}
locals {}
`,
			expectIssue: true,
			issueRule:   "block_order",
		},
		{
			name: "wrong order - locals before include",
			content: `locals {}
include "root" {}
terraform {}
`,
			expectIssue: true,
			issueRule:   "block_order",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasBlockOrderIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "block_order" {
					hasBlockOrderIssue = true
					break
				}
			}

			if tt.expectIssue && !hasBlockOrderIssue {
				t.Error("expected block_order issue, got none")
			}
			if !tt.expectIssue && hasBlockOrderIssue {
				t.Errorf("unexpected block_order issue: %v", result.Issues)
			}
		})
	}
}

func TestNameValidationRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"name_validation": {
				"enabled": true,
				"pattern": "^[a-z][a-z0-9_]*$",
				"blocks": ["include", "dependency"]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "valid name with underscore",
			content: `include "vault_azuread" {}
`,
			expectIssue: false,
		},
		{
			name: "valid name simple",
			content: `include "vpc" {}
`,
			expectIssue: false,
		},
		{
			name: "invalid name with hyphen",
			content: `include "vault-azuread" {}
`,
			expectIssue: true,
		},
		{
			name: "invalid dependency name",
			content: `dependency "my-vpc" {}
`,
			expectIssue: true,
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasNameValidationIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "name_validation" {
					hasNameValidationIssue = true
					break
				}
			}

			if tt.expectIssue && !hasNameValidationIssue {
				t.Error("expected name_validation issue, got none")
			}
			if !tt.expectIssue && hasNameValidationIssue {
				t.Errorf("unexpected name_validation issue: %v", result.Issues)
			}
		})
	}
}

func TestDuplicatesRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"duplicates": {
				"enabled": true,
				"blocks": ["dependency", "include"]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "no duplicates",
			content: `dependency "vpc" {}
dependency "db" {}
`,
			expectIssue: false,
		},
		{
			name: "duplicate dependency",
			content: `dependency "vpc" {}
dependency "vpc" {}
`,
			expectIssue: true,
		},
		{
			name: "duplicate include",
			content: `include "root" {}
include "root" {}
`,
			expectIssue: true,
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasDuplicateIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "duplicates" {
					hasDuplicateIssue = true
					break
				}
			}

			if tt.expectIssue && !hasDuplicateIssue {
				t.Error("expected duplicates issue, got none")
			}
			if !tt.expectIssue && hasDuplicateIssue {
				t.Errorf("unexpected duplicates issue: %v", result.Issues)
			}
		})
	}
}

func TestRequiredFieldsRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"required_fields": {
				"include": {
					"expose": true
				}
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "include with expose",
			content: `include "root" {
  expose = true
}
`,
			expectIssue: false,
		},
		{
			name: "include without expose",
			content: `include "root" {
  path = "..."
}
`,
			expectIssue: true,
		},
		{
			name: "include without expose multiline",
			content: `include "root" {
  path = "some/path"
}
`,
			expectIssue: true,
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "required_fields" {
					hasIssue = true
					break
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected required_fields issue, got %v", result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected required_fields issue: %v", result.Issues)
			}
		})
	}
}

func TestLintTerraformMissingSource(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"required_blocks": {
				"required": [
					{
						"type": "terraform",
						"count": "once",
						"error": "missing terraform block"
					}
				]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	input := `include "root" {
  path = find_in_parent_folders()
}

inputs = {}
`

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	hasError := false
	for _, issue := range result.Issues {
		if issue.Rule == "required_blocks" && issue.Message == "missing terraform block" {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("expected required_blocks error for missing terraform")
	}
}

func TestDependencyPathExistsRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	existingDir := filepath.Join(tmpDir, "existing-module")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"dependency_path_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name: "dependency with existing path",
			content: `dependency "vpc" {
  config_path = "existing-module"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "dependency with non-existing path",
			content: `dependency "vpc" {
  config_path = "non-existent-module"
}
`,
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name: "dependency with parent path",
			content: `dependency "vpc" {
  config_path = "../vpc"
}
`,
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name: "multiple dependencies - one missing",
			content: `dependency "vpc" {
  config_path = "existing-module"
}

dependency "db" {
  config_path = "missing-db"
}
`,
			expectIssue:   true,
			issueContains: "missing-db",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "dependency_path_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected dependency_path_exists issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected dependency_path_exists issue: %v", result.Issues)
			}
		})
	}
}

func TestIncludePathExistsRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	existingDir := filepath.Join(tmpDir, "existing-parent")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"include_path_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name: "include with existing path",
			content: `include "root" {
  path = "existing-parent"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "include with non-existing path",
			content: `include "root" {
  path = "non-existent"
}
`,
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name: "include with function call - skipped",
			content: `include "root" {
  path = find_in_parent_folders()
}
`,
			expectIssue:   false,
			issueContains: "",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "include_path_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected include_path_exists issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected include_path_exists issue: %v", result.Issues)
			}
		})
	}
}

func TestRemoteStateConfigRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"remote_state_config": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

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
    config {
      bucket = "my-bucket"
      key    = "state"
    }
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
    config {
      bucket = "my-bucket"
    }
  }
}
`,
			expectIssue: true,
		},
		{
			name: "terraform without remote_state",
			content: `terraform {
  source = "./module"
}
`,
			expectIssue: false,
		},
		{
			name: "empty remote_state block",
			content: `terraform {
  source = "./module"
  remote_state {}
}
`,
			expectIssue: true,
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "remote_state_config" {
					hasIssue = true
					break
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected remote_state_config issue, got %v", result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected remote_state_config issue: %v", result.Issues)
			}
		})
	}
}

func TestFindInParentFoldersRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	parentDir := filepath.Join(tmpDir, "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `{
		"rules": {
			"terragrunt_functions": {
				"enabled": true,
				"find_in_parent_folders_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	childDir := filepath.Join(parentDir, "child")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}

	terragruntInParent := filepath.Join(parentDir, "terragrunt.hcl")
	if err := os.WriteFile(terragruntInParent, []byte(`terraform {}`), 0o644); err != nil {
		t.Fatal(err)
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
			expectIssue:   false,
			issueContains: "",
		},
		{
			name:     "find_in_parent_folders with custom file in parent",
			testFile: filepath.Join(childDir, "terragrunt.hcl"),
			content: `inputs = {
  config = find_in_parent_folders("terragrunt.hcl")
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name:     "find_in_parent_folders with missing file",
			testFile: filepath.Join(tmpDir, "terragrunt.hcl"),
			content: `include "root" {
  path = find_in_parent_folders("missing.hcl")
}
`,
			expectIssue:   true,
			issueContains: "could not find file",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tt.testFile, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(tt.testFile)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "find_in_parent_folders_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected find_in_parent_folders_exists issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected find_in_parent_folders_exists issue: %v", result.Issues)
			}
		})
	}
}

func TestGetEnvHasDefaultRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terragrunt_functions": {
				"enabled": true,
				"get_env_has_default": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name        string
		content     string
		expectIssue bool
		severity    string
	}{
		{
			name: "get_env with default value",
			content: `locals {
  env = get_env("ENV", "dev")
}
`,
			expectIssue: false,
		},
		{
			name: "get_env with default and extra args",
			content: `locals {
  env = get_env("ENV", "dev", "extra")
}
`,
			expectIssue: false,
		},
		{
			name: "get_env without default value",
			content: `locals {
  env = get_env("ENV")
}
`,
			expectIssue: true,
			severity:    "warning",
		},
		{
			name: "multiple get_env mixed",
			content: `locals {
  env   = get_env("ENV", "dev")
  region = get_env("REGION")
}
`,
			expectIssue: true,
			severity:    "warning",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "get_env_has_default" {
					hasIssue = true
					break
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected get_env_has_default issue, got %v", result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected get_env_has_default issue: %v", result.Issues)
			}
		})
	}
}

func TestDependencyPathExistsWithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"dependency_path_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	absPath := existingDir
	content := fmt.Sprintf(`dependency "vpc" {
  config_path = "%s"
}
`, absPath)

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Rule == "dependency_path_exists" {
			t.Errorf("unexpected dependency_path_exists issue: %v", issue.Message)
		}
	}
}

func TestIncludePathExistsWithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"include_path_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	absPath := existingDir
	content := fmt.Sprintf(`include "root" {
  path = "%s"
}
`, absPath)

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Rule == "include_path_exists" {
			t.Errorf("unexpected include_path_exists issue: %v", issue.Message)
		}
	}
}

func TestFindInParentFoldersInTerraformBlock(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terragrunt_functions": {
				"enabled": true,
				"find_in_parent_folders_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	content := `terraform {
  source = find_in_parent_folders()
}
`

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Rule == "find_in_parent_folders_exists" {
			t.Errorf("unexpected find_in_parent_folders_exists issue in terraform block: %v", issue.Message)
		}
	}
}

func TestGetEnvInInputsBlock(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terragrunt_functions": {
				"enabled": true,
				"get_env_has_default": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	content := `inputs = {
  env     = get_env("ENV", "dev")
  region  = get_env("REGION")
  timeout = 30
}
`

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	issueCount := 0
	for _, issue := range result.Issues {
		if issue.Rule == "get_env_has_default" {
			issueCount++
		}
	}

	if issueCount != 1 {
		t.Errorf("expected 1 get_env_has_default issue for REGION, got %d", issueCount)
	}
}

func TestMultipleDependenciesWithMixedPaths(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"dependency_path_exists": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	content := `dependency "vpc" {
  config_path = "vpc"
}

dependency "db" {
  config_path = "non-existent-db"
}
`

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	issues := 0
	for _, issue := range result.Issues {
		if issue.Rule == "dependency_path_exists" {
			issues++
			if !strings.Contains(issue.Message, "non-existent-db") {
				t.Errorf("issue should mention non-existent-db, got: %v", issue.Message)
			}
		}
	}

	if issues != 1 {
		t.Errorf("expected 1 issue for non-existent-db, got %d", issues)
	}
}

func TestRemoteStateConfigWithBackendAndEmptyString(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terragrunt": {
				"enabled": true,
				"remote_state_config": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	content := `terraform {
  source = "./module"
  remote_state {
    backend = ""
    config {
      bucket = "my-bucket"
    }
  }
}
`

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	hasIssue := false
	for _, issue := range result.Issues {
		if issue.Rule == "remote_state_config" {
			hasIssue = true
		}
	}

	if !hasIssue {
		t.Error("expected remote_state_config issue for empty backend string")
	}
}

func TestTerraformSourceRequired(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terraform_block": {
				"enabled": true,
				"source_required": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name: "terraform block with source",
			content: `terraform {
  source = "./module"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "terraform block without source",
			content: `terraform {
  version = "1.0.0"
}
`,
			expectIssue:   true,
			issueContains: "must have 'source' attribute",
		},
		{
			name: "no terraform block",
			content: `locals {
  name = "test"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "terraform_source_required" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_source_required issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_source_required issue: %v", result.Issues)
			}
		})
	}
}

func TestTerraformVersionFormat(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terraform_block": {
				"enabled": true,
				"version_format": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name: "valid version",
			content: `terraform {
  version = "1.0.0"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "valid version with v prefix",
			content: `terraform {
  version = "v1.5.2"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "invalid version format",
			content: `terraform {
  version = "latest"
}
`,
			expectIssue:   true,
			issueContains: "may not match expected format",
		},
		{
			name: "valid required_version",
			content: `terraform {
  required_version = ">= 1.0.0"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "valid required_version with constraints",
			content: `terraform {
  required_version = ">= 1.0.0, < 2.0.0"
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "invalid required_version format",
			content: `terraform {
  required_version = "1.x"
}
`,
			expectIssue:   true,
			issueContains: "may not match expected format",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "terraform_version_format" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_version_format issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_version_format issue: %v", result.Issues)
			}
		})
	}
}

func TestTerraformExtraArguments(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terraform_block": {
				"enabled": true,
				"extra_arguments_valid": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name: "extra_arguments with name",
			content: `terraform {
  extra_arguments "example" {
    name = "example"
  }
}
`,
			expectIssue:   true,
			issueContains: "should have 'arguments' or nested blocks",
		},
		{
			name: "extra_arguments with arguments",
			content: `terraform {
  extra_arguments "example" {
    arguments = ["-var", "foo=bar"]
  }
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "extra_arguments with nested block",
			content: `terraform {
  extra_arguments "example" {
    cli_config {
      name = "test"
    }
  }
}
`,
			expectIssue:   false,
			issueContains: "",
		},
		{
			name: "extra_arguments missing name",
			content: `terraform {
  extra_arguments {
    arguments = ["-var", "foo=bar"]
  }
}
`,
			expectIssue:   true,
			issueContains: "should have a non-empty 'name' attribute",
		},
		{
			name: "extra_arguments empty",
			content: `terraform {
  extra_arguments "example" {}
}
`,
			expectIssue:   true,
			issueContains: "should have 'arguments' or nested blocks",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "terraform_extra_arguments_valid" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_extra_arguments_valid issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_extra_arguments_valid issue: %v", result.Issues)
			}
		})
	}
}

func TestTerraformDeprecatedFields(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terraform_block": {
				"enabled": true,
				"no_deprecated_fields": true
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name: "deprecated terraform field",
			content: `terraform {
  terraform {
    source = "./module"
  }
}
`,
			expectIssue:   true,
			issueContains: "block 'terraform' is deprecated",
		},
		{
			name: "deprecated before_hook block",
			content: `terraform {
  before_hook {
    commands = ["echo hello"]
  }
}
`,
			expectIssue:   true,
			issueContains: "block 'before_hook' is deprecated",
		},
		{
			name: "deprecated after_hook block",
			content: `terraform {
  after_hook {
    commands = ["echo hello"]
  }
}
`,
			expectIssue:   true,
			issueContains: "block 'after_hook' is deprecated",
		},
		{
			name: "valid modern fields",
			content: `terraform {
  before_hooks {
    hooks {
      commands = ["echo hello"]
    }
  }
  after_hooks {
    hooks {
      commands = ["echo goodbye"]
    }
  }
}
`,
			expectIssue:   false,
			issueContains: "",
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "terraform_deprecated_fields" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_deprecated_fields issue containing %q, got %v", tt.issueContains, result.Issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_deprecated_fields issue: %v", result.Issues)
			}
		})
	}
}

func TestFunctionNotEvaluatedWhenDisabled(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"terragrunt_functions": {
				"enabled": true,
				"find_in_parent_folders_exists": false,
				"get_env_has_default": false
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	content := `include "root" {
  path = find_in_parent_folders("non-existent.hcl")
}

locals {
  env = get_env("MISSING_DEFAULT")
}
`

	file := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Rule == "find_in_parent_folders_exists" || issue.Rule == "get_env_has_default" {
			t.Errorf("unexpected issue when rule is disabled: %v", issue.Rule)
		}
	}
}
