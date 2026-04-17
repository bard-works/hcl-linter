package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader("/path/to/config")
	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
	if loader.configDir != "/path/to/config" {
		t.Errorf("expected configDir /path/to/config, got %s", loader.configDir)
	}
}

func TestHasSpecificConfigForFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "terragrunt.json")
	if err := os.WriteFile(configFile, []byte(`{"rules":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)

	if !loader.HasSpecificConfigForFile("terragrunt.hcl") {
		t.Error("expected HasSpecificConfigForFile to return true for terragrunt.hcl")
	}

	if loader.HasSpecificConfigForFile("other.hcl") {
		t.Error("expected HasSpecificConfigForFile to return false for other.hcl")
	}
}

func TestHasConfigForFile(t *testing.T) {
	tmpDir := t.TempDir()
	defaultFile := filepath.Join(tmpDir, "default.json")
	if err := os.WriteFile(defaultFile, []byte(`{"rules":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)

	if !loader.HasConfigForFile("anyfile.hcl") {
		t.Error("expected HasConfigForFile to return true when default.json exists")
	}
}

func TestHasConfigForFileWithNilLoader(t *testing.T) {
	var loader *Loader
	if loader.HasConfigForFile("test.hcl") {
		t.Error("expected HasConfigForFile to return false for nil loader")
	}
}

func TestLoadForFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "terragrunt.json")
	configContent := `{
		"rules": {
			"block_order": {
				"enabled": true,
				"order": ["include", "locals"]
			}
		}
	}`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.BlockOrder == nil {
		t.Fatal("BlockOrder should not be nil")
	}
	if !rules.BlockOrder.Enabled {
		t.Error("BlockOrder.Enabled should be true")
	}
	if len(rules.BlockOrder.Order) != 2 {
		t.Errorf("expected order length 2, got %d", len(rules.BlockOrder.Order))
	}
}

func TestLoadForFileFallsBackToDefault(t *testing.T) {
	tmpDir := t.TempDir()
	defaultFile := filepath.Join(tmpDir, "default.json")
	configContent := `{
		"rules": {
			"duplicates": {
				"enabled": true,
				"blocks": ["dependency"]
			}
		}
	}`
	if err := os.WriteFile(defaultFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("unknown.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.Duplicates == nil {
		t.Fatal("Duplicates should not be nil when using default.json")
	}
	if !rules.Duplicates.Enabled {
		t.Error("Duplicates.Enabled should be true from default.json")
	}
}

func TestConfigSourcePrecedence(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() (string, func())
		expect ConfigSource
	}{
		{
			name: "CLI takes precedence",
			setup: func() (string, func()) {
				tmpDir := t.TempDir()
				cliFile := filepath.Join(tmpDir, "test.json")
				os.WriteFile(cliFile, []byte(`{"rules":{}}`), 0644)
				return cliFile, func() {}
			},
			expect: ConfigSourceCLI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			explicitDir, cleanup := tt.setup()
			defer cleanup()

			loader, result := NewLoaderWithDiscovery(explicitDir)
			if result.Source != tt.expect {
				t.Errorf("expected source %v, got %v", tt.expect, result.Source)
			}
			if loader == nil {
				t.Error("loader should not be nil")
			}
		})
	}
}

func TestRulesIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		rules    Rules
		expected bool
	}{
		{
			name:     "empty rules",
			rules:    Rules{},
			expected: false,
		},
		{
			name: "block_order enabled",
			rules: Rules{
				BlockOrder: &BlockOrderConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "array_format enabled",
			rules: Rules{
				ArrayFormat: &ArrayFormatConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "name_validation enabled",
			rules: Rules{
				NameValidation: &NameValidationConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "duplicates enabled",
			rules: Rules{
				Duplicates: &DuplicatesConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "required_fields set",
			rules: Rules{
				RequiredFields: &RequiredFieldsConfig{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rules.IsEnabled()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfigSourceString(t *testing.T) {
	tests := []struct {
		source ConfigSource
		expect string
	}{
		{ConfigSourceNone, "none"},
		{ConfigSourceProject, "project"},
		{ConfigSourceHome, "~/.hcl-linter"},
		{ConfigSourceCwd, ".hcl-linter"},
		{ConfigSourceEnv, "HCL_LINTER_CONFIG_DIR"},
		{ConfigSourceCLI, "--config-source"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			if tt.source.String() != tt.expect {
				t.Errorf("expected %s, got %s", tt.expect, tt.source.String())
			}
		})
	}
}
