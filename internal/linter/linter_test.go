package linter

import (
	"os"
	"path/filepath"
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
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
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
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
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
	if err := os.WriteFile(file, []byte(`locals {}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := l.LintFile(file)
	if err == nil {
		t.Error("expected error for file without config")
	}
}

func TestLintFilesConcurrent(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["terraform"]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "default.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	files := []string{
		filepath.Join(tmpDir, "file1.hcl"),
		filepath.Join(tmpDir, "file2.hcl"),
		filepath.Join(tmpDir, "file3.hcl"),
	}

	for i, file := range files {
		content := "terraform {}\n"
		if i == 1 {
			content = "locals {}\n"
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results := l.LintFiles(files, 2)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for _, result := range results {
		if result == nil {
			t.Error("expected non-nil result")
		}
	}
}

func TestLintFilesNoConcurrency(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["terraform"]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "default.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	file := filepath.Join(tmpDir, "test.hcl")
	if err := os.WriteFile(file, []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results := l.LintFiles([]string{file}, 0)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRequiredBlocksRule(t *testing.T) {
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

	tests := []struct {
		name        string
		content     string
		expectIssue bool
		issueCount  int
	}{
		{
			name:        "has terraform block",
			content:     "terraform {}\n",
			expectIssue: false,
			issueCount:  0,
		},
		{
			name:        "missing terraform block",
			content:     "locals {}\n",
			expectIssue: true,
			issueCount:  1,
		},
		{
			name:        "empty file",
			content:     "",
			expectIssue: true,
			issueCount:  1,
		},
		{
			name:        "multiple blocks but no terraform",
			content:     "include {}\nlocals {}\n",
			expectIssue: true,
			issueCount:  1,
		},
		{
			name:        "multiple terraform blocks - also an issue",
			content:     "terraform {}\nterraform {}\n",
			expectIssue: true,
			issueCount:  1,
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

			hasRequiredBlocksIssue := false
			requiredBlocksCount := 0
			for _, issue := range result.Issues {
				if issue.Rule == "required_blocks" {
					hasRequiredBlocksIssue = true
					requiredBlocksCount++
				}
			}

			if tt.expectIssue && !hasRequiredBlocksIssue {
				t.Error("expected required_blocks issue, got none")
			}
			if !tt.expectIssue && hasRequiredBlocksIssue {
				t.Errorf("unexpected required_blocks issue: %v", result.Issues)
			}
			if requiredBlocksCount != tt.issueCount {
				t.Errorf("expected %d required_blocks issues, got %d", tt.issueCount, requiredBlocksCount)
			}
		})
	}
}

func TestRequiredBlocksRuleCustomMessage(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"required_blocks": {
				"required": [
					{
						"type": "terraform",
						"count": "once",
						"error": "terragrunt files must have a terraform block"
					}
				]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "config.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	file := filepath.Join(tmpDir, "config.hcl")
	if err := os.WriteFile(file, []byte(`locals {}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := l.LintFile(file)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}

	if result.Issues[0].Message != "terragrunt files must have a terraform block" {
		t.Errorf("unexpected error message: %s", result.Issues[0].Message)
	}
}

func TestRequiredBlocksRuleMultipleRequirements(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	configContent := `{
		"rules": {
			"required_blocks": {
				"required": [
					{
						"type": "terraform",
						"count": "once",
						"error": "missing terraform"
					},
					{
						"type": "include",
						"count": "once",
						"error": "missing include"
					}
				]
			}
		}
	}`
	setupTestConfig(t, tmpDir, "terragrunt.json", configContent)

	loader := config.NewLoader(tmpDir)
	l := NewLinter(loader)

	tests := []struct {
		name       string
		content    string
		issueCount int
		messages   []string
	}{
		{
			name:       "has both blocks",
			content:    "terraform {}\ninclude {}\n",
			issueCount: 0,
		},
		{
			name:       "missing terraform only",
			content:    "include {}\n",
			issueCount: 1,
			messages:   []string{"missing terraform"},
		},
		{
			name:       "missing include only",
			content:    "terraform {}\n",
			issueCount: 1,
			messages:   []string{"missing include"},
		},
		{
			name:       "missing both",
			content:    "locals {}\n",
			issueCount: 2,
		},
	}

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

			requiredBlocksIssues := 0
			for _, issue := range result.Issues {
				if issue.Rule == "required_blocks" {
					requiredBlocksIssues++
				}
			}

			if requiredBlocksIssues != tt.issueCount {
				t.Errorf("expected %d required_blocks issues, got %d", tt.issueCount, requiredBlocksIssues)
			}
		})
	}
}
