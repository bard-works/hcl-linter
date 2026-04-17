package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/papaya/hcl-linter/internal/config"
)

func createFixTestConfigDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	return tmpDir
}

func setupFixTestConfig(t *testing.T, tmpDir string, content string) {
	t.Helper()
	configFile := filepath.Join(tmpDir, "terragrunt.json")
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createHCLFile(t *testing.T, tmpDir string, filename string, content string) string {
	t.Helper()
	file := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestFixerFixFile(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform"]
			},
			"required_fields": {
				"include": {
					"expose": true
				}
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	tests := []struct {
		name         string
		content      string
		expectOrder  bool
		expectExpose bool
	}{
		{
			name:         "fix block order",
			content:      "locals {}\ninclude \"root\" {}\nterraform {}\n",
			expectOrder:  true,
			expectExpose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createHCLFile(t, tmpDir, "terragrunt.hcl", tt.content)

			result, err := fixer.FixFile(file)
			if err != nil {
				t.Fatalf("FixFile failed: %v", err)
			}

			if tt.expectOrder || tt.expectExpose {
				if result.Changes == 0 {
					t.Error("expected changes, got 0")
				}
			}

			if !result.Success {
				t.Error("expected Success=true")
			}
		})
	}
}

func TestFixBlockOrder(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform"]
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	tests := []struct {
		name            string
		input           string
		expectReordered bool
	}{
		{
			name:            "reorder blocks - blocks should move",
			input:           "locals {}\ninclude \"root\" {}\nterraform {}\n",
			expectReordered: true,
		},
		{
			name:            "with multiline block",
			input:           "terraform {}\ninclude \"root\" {\n  path = \"...\"\n}\nlocals {}\n",
			expectReordered: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createHCLFile(t, tmpDir, "terragrunt.hcl", tt.input)

			result, err := fixer.FixFile(file)
			if err != nil {
				t.Fatalf("FixFile failed: %v", err)
			}

			if result.Changes == 0 && tt.expectReordered {
				t.Error("expected reordering, got 0 changes")
			}

			// Verify include block comes before terraform
			if strings.Index(result.Content, "include") > strings.Index(result.Content, "terraform") {
				t.Error("expected include to come before terraform")
			}
		})
	}
}

func TestFixNameValidation(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"name_validation": {
				"enabled": true,
				"pattern": "^[a-z][a-z0-9_]*$",
				"blocks": ["include", "dependency"]
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	tests := []struct {
		name          string
		input         string
		expectChange  bool
		checkContains string
	}{
		{
			name: "fix hyphenated include",
			input: `include "vault-azuread" {}
`,
			expectChange:  true,
			checkContains: "vault_azuread",
		},
		{
			name: "fix hyphenated dependency",
			input: `dependency "my-vpc" {}
`,
			expectChange:  true,
			checkContains: "my_vpc",
		},
		{
			name: "already valid",
			input: `include "valid_name" {}
`,
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createHCLFile(t, tmpDir, "terragrunt.hcl", tt.input)

			result, err := fixer.FixFile(file)
			if err != nil {
				t.Fatalf("FixFile failed: %v", err)
			}

			if tt.expectChange && result.Changes == 0 {
				t.Error("expected changes, got 0")
			}

			if tt.checkContains != "" && !strings.Contains(result.Content, tt.checkContains) {
				t.Errorf("expected content to contain %q", tt.checkContains)
			}
		})
	}
}

func TestFixRequiredFields(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"required_fields": {
				"include": {
					"expose": true
				}
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	tests := []struct {
		name          string
		input         string
		expectChange  bool
		checkContains string
	}{
		{
			name: "add expose to single-line block",
			input: `include "root" {}
`,
			expectChange:  true,
			checkContains: "expose = true",
		},
		{
			name: "add expose to multi-line block",
			input: `include "root" {
  path = "..."
}
`,
			expectChange:  true,
			checkContains: "expose = true",
		},
		{
			name: "already has expose",
			input: `include "root" {
  expose = true
}
`,
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createHCLFile(t, tmpDir, "terragrunt.hcl", tt.input)

			result, err := fixer.FixFile(file)
			if err != nil {
				t.Fatalf("FixFile failed: %v", err)
			}

			if tt.expectChange && result.Changes == 0 {
				t.Error("expected changes, got 0")
			}

			if tt.checkContains != "" && !strings.Contains(result.Content, tt.checkContains) {
				t.Errorf("expected content to contain %q, got:\n%s", tt.checkContains, result.Content)
			}
		})
	}
}

func TestFixFileNoConfig(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)
	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	file := createHCLFile(t, tmpDir, "noconfig.hcl", "locals {}")

	_, err := fixer.FixFile(file)
	if err == nil {
		t.Error("expected error for file without config")
	}
}

func TestFixFileMultipleIssues(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform"]
			},
			"name_validation": {
				"enabled": true,
				"pattern": "^[a-z][a-z0-9_]*$",
				"blocks": ["include"]
			},
			"required_fields": {
				"include": {
					"expose": true
				}
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `locals {}
include "vault-azuread" {}
terraform {}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes == 0 {
		t.Error("expected multiple changes")
	}

	if !strings.Contains(result.Content, "include \"vault_azuread\"") {
		t.Error("expected hyphen to be replaced with underscore")
	}

	if !strings.Contains(result.Content, "expose = true") {
		t.Error("expected expose to be added")
	}
}
