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
	configFile := filepath.Join(tmpDir, "terragrunt.hcl")
	if err := os.WriteFile(configFile, []byte("rules {}"), 0o644); err != nil {
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
	defaultFile := filepath.Join(tmpDir, "default.hcl")
	if err := os.WriteFile(defaultFile, []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)

	if !loader.HasConfigForFile("anyfile.hcl") {
		t.Error("expected HasConfigForFile to return true when default.hcl exists")
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
	configFile := filepath.Join(tmpDir, "terragrunt.hcl")
	configContent := `rules {
  block_order {
    enabled = true
    order   = ["include", "locals"]
  }
}`
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
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
	defaultFile := filepath.Join(tmpDir, "default.hcl")
	configContent := `rules {
  duplicates {
    enabled = true
    blocks  = ["dependency"]
  }
}`
	if err := os.WriteFile(defaultFile, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("unknown.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.Duplicates == nil {
		t.Fatal("Duplicates should not be nil when using default.hcl")
	}
	if !rules.Duplicates.Enabled {
		t.Error("Duplicates.Enabled should be true from default.hcl")
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
				configDir := filepath.Join(tmpDir, ".hcl-linter")
				if err := os.MkdirAll(configDir, 0o755); err != nil {
					t.Fatal(err)
				}

				cliFile := filepath.Join(configDir, "test.hcl")
				if err := os.WriteFile(cliFile, []byte("rules {}"), 0o644); err != nil {
					t.Fatal(err)
				}

				return tmpDir, func() {}
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

func TestLoadDependencyPathsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "terragrunt.hcl")
	configContent := `rules {
  dependency_paths {
    enabled = true
  }

  include_paths {
    enabled = true
  }

  remote_state {
    enabled         = true
    require_backend = true
  }
}`
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.DependencyPaths == nil || !rules.DependencyPaths.Enabled {
		t.Error("DependencyPaths should be enabled")
	}
	if rules.IncludePaths == nil || !rules.IncludePaths.Enabled {
		t.Error("IncludePaths should be enabled")
	}
	if rules.RemoteState == nil || !rules.RemoteState.Enabled || !rules.RemoteState.RequireBackend {
		t.Errorf("RemoteState should be enabled with RequireBackend, got %+v", rules.RemoteState)
	}
}

func TestLoadHCLFunctionsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "terragrunt.hcl")
	configContent := `rules {
  hcl_functions {
    enabled                       = true
    find_in_parent_folders_exists = true
    get_env_has_default           = true
  }
}`
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.HCLFunctions == nil {
		t.Fatal("HCLFunctions should not be nil")
	}
	if !rules.HCLFunctions.Enabled {
		t.Error("HCLFunctions.Enabled should be true")
	}
	if !rules.HCLFunctions.FindInParentFoldersExists {
		t.Error("HCLFunctions.FindInParentFoldersExists should be true")
	}
	if !rules.HCLFunctions.GetEnvHasDefault {
		t.Error("HCLFunctions.GetEnvHasDefault should be true")
	}
}

func TestLoadTerraformBlockConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "terragrunt.hcl")
	configContent := `rules {
  terraform_block {
    enabled               = true
    source_required       = true
    version_format        = true
    extra_arguments_valid = true
    no_deprecated_fields  = true
  }
}`
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.TerraformBlock == nil {
		t.Fatal("TerraformBlock should not be nil")
	}
	if !rules.TerraformBlock.Enabled {
		t.Error("TerraformBlock.Enabled should be true")
	}
	if !rules.TerraformBlock.SourceRequired {
		t.Error("TerraformBlock.SourceRequired should be true")
	}
	if !rules.TerraformBlock.VersionFormat {
		t.Error("TerraformBlock.VersionFormat should be true")
	}
	if !rules.TerraformBlock.ExtraArgumentsValid {
		t.Error("TerraformBlock.ExtraArgumentsValid should be true")
	}
	if !rules.TerraformBlock.NoDeprecatedFields {
		t.Error("TerraformBlock.NoDeprecatedFields should be true")
	}
}

func TestLoadTerraformBlockHCLConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "terragrunt.hcl")
	configContent := `
rules {
  terraform_block {
    enabled = true
    source_required = true
    version_format = false
    extra_arguments_valid = true
    no_deprecated_fields = false
  }
}
`
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.TerraformBlock == nil {
		t.Fatal("TerraformBlock should not be nil")
	}
	if !rules.TerraformBlock.Enabled {
		t.Error("TerraformBlock.Enabled should be true")
	}
	if rules.TerraformBlock.VersionFormat {
		t.Error("TerraformBlock.VersionFormat should be false")
	}
}

func TestExtendsMultiLevelChain(t *testing.T) {
	tmpDir := t.TempDir()

	grandparentContent := `rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}`
	parentContent := `extends = "grandparent"

rules {
  duplicates {
    enabled = true
    blocks  = ["dependency"]
  }
}`
	childContent := `extends = "parent"

rules {
  block_order {
    enabled = true
    order   = ["include", "locals"]
  }
}`

	for name, content := range map[string]string{
		"grandparent.hcl": grandparentContent,
		"parent.hcl":      parentContent,
		"child.hcl":       childContent,
	} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("child.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.BlankLines == nil {
		t.Fatal("BlankLines should be inherited from grandparent")
	}
	if rules.Duplicates == nil {
		t.Fatal("Duplicates should be inherited from parent")
	}
	if rules.BlockOrder == nil {
		t.Fatal("BlockOrder should be set in child")
	}
	if len(rules.BlockOrder.Order) != 2 {
		t.Errorf("expected 2 items in block_order, got %d", len(rules.BlockOrder.Order))
	}
}

func TestExtendsMissingBase(t *testing.T) {
	tmpDir := t.TempDir()

	childContent := `extends = "nonexistent"
rules {}`

	if err := os.WriteFile(filepath.Join(tmpDir, "child.hcl"), []byte(childContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	_, err := loader.LoadForFile("child.hcl")
	if err == nil {
		t.Fatal("expected error when base config does not exist")
	}
}

func TestExtendsWithExplicitHCLExtension(t *testing.T) {
	tmpDir := t.TempDir()

	baseContent := `rules {
  duplicates {
    enabled = true
    blocks  = ["dependency"]
  }
}`
	childContent := `extends = "base.hcl"
rules {}`

	if err := os.WriteFile(filepath.Join(tmpDir, "base.hcl"), []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "child.hcl"), []byte(childContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("child.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}
	if rules.Duplicates == nil {
		t.Fatal("Duplicates should be inherited from base")
	}
}

func TestExtendsInheritance(t *testing.T) {
	tmpDir := t.TempDir()

	baseContent := `rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
  duplicates {
    enabled = true
    blocks  = ["dependency"]
  }
}`
	childContent := `extends = "base"

rules {
  block_order {
    enabled = true
    order   = ["locals", "terraform"]
  }
}`

	if err := os.WriteFile(filepath.Join(tmpDir, "base.hcl"), []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "child.hcl"), []byte(childContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("child.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.BlockOrder == nil {
		t.Fatal("BlockOrder should not be nil")
	}
	if len(rules.BlockOrder.Order) != 2 || rules.BlockOrder.Order[0] != "locals" {
		t.Errorf("expected child block_order to override base, got %v", rules.BlockOrder.Order)
	}

	if rules.Duplicates == nil {
		t.Fatal("Duplicates should be inherited from base")
	}
	if len(rules.Duplicates.Blocks) != 1 || rules.Duplicates.Blocks[0] != "dependency" {
		t.Errorf("expected duplicates inherited from base, got %v", rules.Duplicates.Blocks)
	}
}

func TestExtendsCircularDetection(t *testing.T) {
	tmpDir := t.TempDir()

	aContent := `extends = "b"
rules {}`
	bContent := `extends = "a"
rules {}`

	if err := os.WriteFile(filepath.Join(tmpDir, "a.hcl"), []byte(aContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.hcl"), []byte(bContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	_, err := loader.LoadForFile("a.hcl")
	if err == nil {
		t.Fatal("expected error for circular extends")
	}
}

func TestNewLoaderWithDiscoveryCwd(t *testing.T) {
	// Change working directory to a tmpDir that contains .hcl-linter/
	// so getCwdConfigDir discovery fires the ConfigSourceCwd branch.
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Setenv("HCL_LINTER_CONFIG_DIR", "")

	_, result := NewLoaderWithDiscovery("")
	if result.Source != ConfigSourceCwd {
		t.Errorf("expected ConfigSourceCwd, got %v", result.Source)
	}
}

func TestNewLoaderWithDiscoveryEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HCL_LINTER_CONFIG_DIR", tmpDir)

	_, result := NewLoaderWithDiscovery("")
	if result.Source != ConfigSourceEnv {
		t.Errorf("expected ConfigSourceEnv, got %v", result.Source)
	}
}

func TestIsWithinDir(t *testing.T) {
	// dir == "" always false
	if isWithinDir("/some/path", "") {
		t.Error("expected false for empty dir")
	}
	// path is same as dir → rel == "."
	if !isWithinDir("/a/b", "/a/b") {
		t.Error("expected true when path == dir")
	}
	// path is inside dir
	if !isWithinDir("/a/b/c", "/a/b") {
		t.Error("expected true when path inside dir")
	}
	// path is outside dir
	if isWithinDir("/x/y", "/a/b") {
		t.Error("expected false when path outside dir")
	}
}

func TestHasConfigForFileEmptyConfigDir(t *testing.T) {
	loader := NewLoader("")
	if loader.HasConfigForFile("test.hcl") {
		t.Error("expected false for loader with empty configDir")
	}
}

func TestLoadForFileNoConfigFound(t *testing.T) {
	loader := NewLoader(t.TempDir()) // empty dir, no config files
	_, err := loader.LoadForFile("noconfig.hcl")
	if err == nil {
		t.Fatal("expected error when no config file and no default exist")
	}
}

func TestLoadForFileUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	// Create file with no extension (unsupported) + valid default.hcl fallback.
	// loadConfigFile("terragrunt") → "unsupported config format" → skip, fallback to default.
	if err := os.WriteFile(filepath.Join(tmpDir, "terragrunt"), []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "default.hcl"), []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	// configBaseName("terragrunt.hcl") = "terragrunt"
	// findConfigFiles tries "terragrunt.hcl" (missing), then "terragrunt" (exists, unsupported)
	// → skip, then default.hcl → success
	_, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("expected successful fallback to default.hcl, got: %v", err)
	}
}

func TestNewLoaderWithDiscoveryNoConfig(t *testing.T) {
	// Clear env var so env resolution fails, pass nonexistent explicit dir.
	// Exercises cwd/home/project discovery branches before returning None.
	t.Setenv("HCL_LINTER_CONFIG_DIR", "")
	loader, result := NewLoaderWithDiscovery(filepath.Join(t.TempDir(), "nonexistent-config"))
	if loader == nil {
		t.Fatal("expected non-nil loader")
	}
	// In a clean CI environment (no .hcl-linter in cwd/home/project) this is None.
	// Accept None or any source — we just confirm no panic and valid result.
	_ = result
}

func TestResolveConfigDirForFileNilReceiver(t *testing.T) {
	var loader *Loader
	result := loader.resolveConfigDirForFile("test.hcl")
	if result != "" {
		t.Errorf("expected empty string for nil loader, got %q", result)
	}
}

func TestWalkForConfigDirReachesRoot(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(filepath.Join(tmpDir, ".hcl-linter"))
	// start = subDir (inside boundary=tmpDir), no .hcl-linter in subDir.
	// Walk: subDir/.hcl-linter missing → parent tmpDir → tmpDir/.hcl-linter == rootAbs → return configDir.
	result := loader.resolveConfigDirForFile(filepath.Join(subDir, "test.hcl"))
	if result != loader.configDir {
		t.Errorf("expected configDir %q, got %q", loader.configDir, result)
	}
}

func TestHasConfigForFileWithSpecificConfig(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "terragrunt.hcl"), []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(tmpDir)
	// Specific config exists → HasConfigForFile returns true (hits the return-true branch).
	if !loader.HasConfigForFile("terragrunt.hcl") {
		t.Error("expected true when specific config file exists")
	}
}

func TestLoadForFileDefaultParseError(t *testing.T) {
	tmpDir := t.TempDir()
	// No specific config; default.hcl has invalid HCL → loadConfigFile returns real parse error.
	if err := os.WriteFile(filepath.Join(tmpDir, "default.hcl"), []byte("{{{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(tmpDir)
	_, err := loader.LoadForFile("terragrunt.hcl")
	if err == nil {
		t.Fatal("expected error for invalid default.hcl")
	}
}

func TestLoadForFileDefaultUnsupportedSkip(t *testing.T) {
	tmpDir := t.TempDir()
	// "default" (no extension) exists with unsupported format → loadConfigFile errors with
	// "unsupported config format" → skip and fall through to "no config found" error.
	if err := os.WriteFile(filepath.Join(tmpDir, "default"), []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(tmpDir)
	_, err := loader.LoadForFile("noconfig.hcl")
	if err == nil {
		t.Fatal("expected error when only unsupported-format default exists")
	}
}

func TestResolveConfigDirForFileCache(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// First call populates cache; second call hits it.
	r1 := loader.resolveConfigDirForFile("test.hcl")
	r2 := loader.resolveConfigDirForFile("test.hcl")
	if r1 != r2 {
		t.Errorf("cached result mismatch: %q != %q", r1, r2)
	}
}

func TestWalkForConfigDirOutsideBoundary(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	loader := NewLoader(filepath.Join(dir1, ".hcl-linter"))

	// File is in dir2 which is outside the boundary rooted at dir1.
	// walkForConfigDir should return l.configDir without walking.
	result := loader.resolveConfigDirForFile(filepath.Join(dir2, "test.hcl"))
	if result != loader.configDir {
		t.Errorf("expected configDir for out-of-boundary file, got %q", result)
	}
}

func TestWalkForConfigDirFindsIntermediate(t *testing.T) {
	// covers `return candidate` in walkForConfigDir:
	// an intermediate .hcl-linter/ exists between start and rootAbs and has a matching config.
	tmpDir := t.TempDir()

	// configDir = tmpDir/.hcl-linter (boundary = tmpDir)
	configDir := filepath.Join(tmpDir, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Intermediate .hcl-linter in tmpDir/sub/ with matching terragrunt.hcl
	subLinter := filepath.Join(tmpDir, "sub", ".hcl-linter")
	if err := os.MkdirAll(subLinter, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subLinter, "terragrunt.hcl"), []byte("rules {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File deep inside tmpDir/sub/ so walk finds subLinter before rootAbs.
	targetFile := filepath.Join(tmpDir, "sub", "deep", "terragrunt.hcl")

	loader := NewLoader(configDir)
	result := loader.resolveConfigDirForFile(targetFile)
	if result != subLinter {
		t.Errorf("expected intermediate .hcl-linter %q, got %q", subLinter, result)
	}
}

func TestWalkForConfigDirReachesBoundary(t *testing.T) {
	// covers `dir == boundary` in walkForConfigDir:
	// configDir is NOT named .hcl-linter so candidate never equals rootAbs at boundary level;
	// walk reaches boundary with no match and returns l.configDir.
	tmpDir := t.TempDir()

	// configDir = tmpDir/configs (boundary = tmpDir)
	configDir := filepath.Join(tmpDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// File inside tmpDir/sub/ — no .hcl-linter dirs anywhere.
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(configDir)
	result := loader.resolveConfigDirForFile(filepath.Join(subDir, "service.hcl"))
	if result != configDir {
		t.Errorf("expected configDir %q at boundary, got %q", configDir, result)
	}
}

func TestExtendsWithDefault(t *testing.T) {
	tmpDir := t.TempDir()

	defaultContent := `rules {
  blank_lines {
    enabled      = true
    within_blocks = true
  }
}`
	childContent := `extends = "default"

rules {
  duplicates {
    enabled = true
    blocks  = ["dependency"]
  }
}`

	if err := os.WriteFile(filepath.Join(tmpDir, "default.hcl"), []byte(defaultContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "terragrunt.hcl"), []byte(childContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile failed: %v", err)
	}

	if rules.BlankLines == nil {
		t.Fatal("BlankLines should be inherited from default")
	}
	if !rules.BlankLines.WithinBlocks {
		t.Error("BlankLines.WithinBlocks should be true (inherited)")
	}
	if rules.Duplicates == nil {
		t.Fatal("Duplicates should be set in child")
	}
}
