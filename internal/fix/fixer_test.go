package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
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
	configDir := createFixTestConfigDir(t)
	loader := config.NewLoader(configDir)
	fixer := NewFixer(loader)

	srcDir := t.TempDir()
	file := createHCLFile(t, srcDir, "noconfig.hcl", "locals {}")

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

func TestFixBlankLinesWithinBlocks(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
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
			name: "remove blank lines in object attribute",
			input: `inputs = {

  repository = "test"

  tags = "value"

}
`,
			expectChange:  true,
			checkContains: "repository = \"test\"",
		},
		{
			name: "single blank line between attributes - preserved",
			input: `inputs = {
  repository = "test"

  tags = "value"
}
`,
			expectChange:  false,
			checkContains: "tags = \"value\"",
		},
		{
			name: "multiple blank lines reduced to one",
			input: `inputs = {
  repository = "test"


  tags = "value"
}
`,
			expectChange:  true,
			checkContains: "tags = \"value\"",
		},
		{
			name: "multiple blank lines between attributes",
			input: `inputs = {



  repository = "test"



  tags = "value"



}
`,
			expectChange:  true,
			checkContains: "tags = \"value\"",
		},
		{
			name: "no blank lines - no change",
			input: `inputs = {
  repository = "test"
  tags = "value"
}
`,
			expectChange: false,
		},
		{
			name: "block ends properly",
			input: `inputs = {

  repository = "test"

}
`,
			expectChange:  true,
			checkContains: "repository = \"test\"",
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

			if !tt.expectChange && result.Changes > 0 {
				t.Errorf("expected no changes, got %d", result.Changes)
			}

			if tt.checkContains != "" && !strings.Contains(result.Content, tt.checkContains) {
				t.Errorf("expected content to contain %q, got:\n%s", tt.checkContains, result.Content)
			}
		})
	}
}

func TestFixBlankLinesNestedBlocks(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	tests := []struct {
		name         string
		input        string
		expectChange bool
	}{
		{
			name: "terraform with nested before_hook - should remove blank lines",
			input: `terraform {

  source = "."

  before_hook "test" {

    commands = ["apply"]

  }

}
`,
			expectChange: true,
		},
		{
			name: "no nested blocks",
			input: `terraform {

  source = "."

}
`,
			expectChange: true,
		},
		{
			name: "deeply nested",
			input: `terraform {

  before_hook "test" {

    before_hook "nested" {

      commands = ["plan"]

    }

  }

}
`,
			expectChange: true,
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

			if !tt.expectChange && result.Changes > 0 {
				t.Errorf("expected no changes, got %d", result.Changes)
			}
		})
	}
}

func TestFixBlankLinesDisabled(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": false,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `inputs = {

  repository = "test"

}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes > 0 {
		t.Errorf("expected no changes when disabled, got %d", result.Changes)
	}
}

func TestFixFilesConcurrent(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	files := []string{
		createHCLFile(t, tmpDir, "file1.hcl", "inputs = {\n\n  a = \"b\"\n\n}\n"),
		createHCLFile(t, tmpDir, "file2.hcl", "inputs = {\n\n  c = \"d\"\n\n}\n"),
		createHCLFile(t, tmpDir, "file3.hcl", "inputs = {\n\n  e = \"f\"\n\n}\n"),
	}

	results := fixer.FixFiles(files, 2)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for _, result := range results {
		if result == nil {
			t.Error("expected non-nil result")
		}
	}
}

func TestFixFilesNoConcurrency(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	file := createHCLFile(t, tmpDir, "test.hcl", "inputs = {\n\n  a = \"b\"\n\n}\n")

	results := fixer.FixFiles([]string{file}, 0)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestFixBlockOrderWithBlankLines(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform", "inputs"]
			},
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	tests := []struct {
		name         string
		input        string
		expectChange bool
	}{
		{
			name: "reorder and remove blank lines",
			input: `terraform {}

include "root" {

  path = "..."

}



inputs = {



  value = "test"



}
`,
			expectChange: true,
		},
		{
			name: "multiple blocks with blank lines",
			input: `inputs {}
terraform {}

locals {}
`,
			expectChange: true,
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
		})
	}
}

func TestFixPreservesComplexExpressions(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform", "dependency", "inputs"]
			},
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `terraform {
  source = "."
}

include "root" {
  path = find_in_parent_folders("root.hcl")
}

inputs = {
  service_details = { for k, v in local.service : k => v if !contains(["sd_helper", "common_tags"], k) }
}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if !strings.Contains(result.Content, "service_details = { for k, v in local.service : k => v if !contains([\"sd_helper\", \"common_tags\"], k) }") {
		t.Error("complex expression was corrupted")
	}

	if strings.Contains(result.Content, "= =") {
		t.Error("double equals sign appeared in output")
	}
}

func TestFixMultipleObjectAttributes(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `inputs = {

  repo = "a"

}

terraform {

  source = "."

  before_hook "test" {

    commands = ["apply"]

  }

}

other = {

  value = "test"

}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes == 0 {
		t.Error("expected changes")
	}

	if strings.Count(result.Content, "inputs = {") != 1 {
		t.Error("duplicate blocks detected")
	}
}

func TestFixTrailingNewline(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `inputs = {

  value = "test"

}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if !strings.HasSuffix(result.Content, "\n") {
		t.Error("output should end with newline")
	}
}

func TestFixNoHangOnComplexFile(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			},
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform", "dependency", "inputs"]
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `inputs = {

  repository = "test"

  tags = local.service.common_tags

}

terraform {

  source = "."

  before_hook "test" {

    commands = ["apply"]

  }

}

include "root" {
  path = find_in_parent_folders("root.hcl")
}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes == 0 {
		t.Error("expected changes")
	}

	lines := strings.Split(result.Content, "\n")
	braceLevel := 0
	prevWasBlank := false
	for i, line := range lines {
		for _, ch := range line {
			switch ch {
			case '{':
				braceLevel++
			case '}':
				braceLevel--
			}
		}
		isBlank := strings.TrimSpace(line) == ""
		if isBlank && i > 0 && i < len(lines)-1 && braceLevel > 0 {
			if prevWasBlank {
				t.Errorf("unexpected duplicate blank line at index %d (inside block with depth %d)", i, braceLevel)
			}
		}
		prevWasBlank = isBlank
	}
}

func TestFixRegressionDuplicateBlocks(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform", "inputs"]
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `terraform {}

include "root" {}

inputs = {}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes == 0 {
		t.Error("expected changes")
	}

	includeCount := strings.Count(result.Content, "include \"root\"")
	if includeCount != 1 {
		t.Errorf("expected 1 include block, got %d", includeCount)
	}

	terraformCount := strings.Count(result.Content, "terraform {")
	if terraformCount != 1 {
		t.Errorf("expected 1 terraform block, got %d", terraformCount)
	}
}

func TestFixTerraformRealisticBlocks(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			},
			"block_order": {
				"enabled": true,
				"order": ["include", "locals", "terraform", "inputs"]
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `locals {}

include "root" {
  path = find_in_parent_folders()
}

terraform {

  source = "../../../modules/ecs-service"

  before_hook "validate" {

    commands = ["validate", "plan"]

  }

  after_hook "apply" {

    commands = ["apply"]

  }

}

inputs = {

  environment = "production"

}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes == 0 {
		t.Error("expected changes for blank lines")
	}

	terraformCount := strings.Count(result.Content, "terraform {")
	if terraformCount != 1 {
		t.Errorf("expected 1 terraform block, got %d", terraformCount)
	}

	if !strings.Contains(result.Content, "before_hook") {
		t.Error("expected before_hook to be preserved")
	}

	if !strings.Contains(result.Content, "after_hook") {
		t.Error("expected after_hook to be preserved")
	}

	if strings.Contains(result.Content, "source = \"../../../modules/ecs-service\"\n\n\n") {
		t.Error("multiple blank lines should be reduced to one")
	}
}

func TestFixTerraformWithRemoteState(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `terraform {

  source = "../../../modules/ec2"

  remote_state {

    backend = "s3"

    config = {

      bucket = "my-terraform-state"

      key    = "ec2/terraform.tfstate"

      region = "us-east-1"

    }

  }

}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if result.Changes == 0 {
		t.Error("expected changes for blank lines")
	}

	if !strings.Contains(result.Content, "remote_state") {
		t.Error("expected remote_state to be preserved")
	}

	if !strings.Contains(result.Content, "backend = \"s3\"") {
		t.Error("expected backend config to be preserved")
	}

	if !strings.Contains(result.Content, "bucket = \"my-terraform-state\"") {
		t.Error("expected bucket config to be preserved")
	}
}

func TestFixTerraformPreservesAllAttributes(t *testing.T) {
	tmpDir := createFixTestConfigDir(t)

	configContent := `{
		"rules": {
			"blank_lines": {
				"enabled": true,
				"within_blocks": true
			}
		}
	}`
	setupFixTestConfig(t, tmpDir, configContent)

	loader := config.NewLoader(tmpDir)
	fixer := NewFixer(loader)

	input := `terraform {

  source = "../../../modules/service"

  before_hook "before_validate" {

    commands = ["validate"]

  }

  before_hook "before_plan" {

    commands = ["plan"]

  }

  after_hook "after_apply" {

    commands = ["apply", "-auto-approve"]

  }

  terraform {
    extra_arguments "common" {
      commands = ["plan", "apply"]
    }
  }

}
`

	file := createHCLFile(t, tmpDir, "terragrunt.hcl", input)

	result, err := fixer.FixFile(file)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	requiredContent := []string{
		"terraform {",
		"source = \"../../../modules/service\"",
		`before_hook "before_validate"`,
		`before_hook "before_plan"`,
		`after_hook "after_apply"`,
		`extra_arguments "common"`,
		`terraform {`,
	}

	for _, content := range requiredContent {
		if !strings.Contains(result.Content, content) {
			t.Errorf("expected content to contain %q", content)
		}
	}
}
