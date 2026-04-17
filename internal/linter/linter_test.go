package linter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/papaya/hcl-linter/internal/config"
)

func createTestConfigDir(t *testing.T) string {
	tmpDir := t.TempDir()
	return tmpDir
}

func setupTestConfig(t *testing.T, tmpDir string, name string, content string) {
	configFile := filepath.Join(tmpDir, name)
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
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
			if err := os.WriteFile(file, []byte(tt.content), 0644); err != nil {
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
			if err := os.WriteFile(file, []byte(tt.content), 0644); err != nil {
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
			if err := os.WriteFile(file, []byte(tt.content), 0644); err != nil {
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
			name: "has expose",
			content: `include "root" {
  expose = true
}
`,
			expectIssue: false,
		},
		{
			name: "missing expose",
			content: `include "root" {}
`,
			expectIssue: true,
		},
		{
			name: "missing expose in multiline",
			content: `include "root" {
  path = "..."
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
			if err := os.WriteFile(file, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasRequiredFieldsIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "required_fields" {
					hasRequiredFieldsIssue = true
					break
				}
			}

			if tt.expectIssue && !hasRequiredFieldsIssue {
				t.Error("expected required_fields issue, got none")
			}
			if !tt.expectIssue && hasRequiredFieldsIssue {
				t.Errorf("unexpected required_fields issue: %v", result.Issues)
			}
		})
	}
}

func TestArrayFormatRule(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"array_format": {
				"enabled": true,
				"multiline_threshold": 2
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
			name: "single item inline",
			content: `inputs = ["a"]
`,
			expectIssue: false,
		},
		{
			name: "multiple items inline - should be multiline",
			content: `inputs = ["a", "b"]
`,
			expectIssue: true,
		},
		{
			name: "with dependency reference - not flagged",
			content: `inputs = [dependency.vpc.outputs.id]
`,
			expectIssue: false,
		},
	}

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			if err := os.WriteFile(file, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := l.LintFile(file)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			hasArrayFormatIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "array_format" {
					hasArrayFormatIssue = true
					break
				}
			}

			if tt.expectIssue && !hasArrayFormatIssue {
				t.Error("expected array_format issue, got none")
			}
			if !tt.expectIssue && hasArrayFormatIssue {
				t.Errorf("unexpected array_format issue: %v", result.Issues)
			}
		})
	}
}

func TestResultHasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected bool
	}{
		{
			name:     "no issues",
			result:   Result{Issues: []Issue{}},
			expected: false,
		},
		{
			name:     "warning only",
			result:   Result{Issues: []Issue{{Severity: SeverityWarning}}},
			expected: false,
		},
		{
			name:     "error present",
			result:   Result{Issues: []Issue{{Severity: SeverityError}}},
			expected: true,
		},
		{
			name:     "info only",
			result:   Result{Issues: []Issue{{Severity: SeverityInfo}}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.HasErrors() != tt.expected {
				t.Errorf("expected HasErrors=%v, got %v", tt.expected, tt.result.HasErrors())
			}
		})
	}
}

func TestResultSummary(t *testing.T) {
	result := Result{
		File:   "/path/to/terragrunt.hcl",
		Issues: []Issue{{}, {}},
	}

	summary := result.Summary()
	if summary != "terragrunt.hcl: 2 issue(s)" {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestLintFileNoConfig(t *testing.T) {
	tmpDir := createTestConfigDir(t)
	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	file := filepath.Join(tmpDir, "noconfig.hcl")
	if err := os.WriteFile(file, []byte(`locals {}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := l.LintFile(file)
	if err == nil {
		t.Error("expected error for file without config")
	}
}
