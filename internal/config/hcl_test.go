package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "terragrunt.hcl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadTestRules(t *testing.T, content string) *Rules {
	t.Helper()
	tmpDir := t.TempDir()
	writeTestConfig(t, tmpDir, content)
	rules, err := NewLoader(tmpDir).LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}
	return rules
}

func TestParseHCLArrayFormat(t *testing.T) {
	rules := loadTestRules(t, `rules {
  array_format {
    enabled             = true
    multiline_threshold = 3
    sort                = true
  }
}`)

	if rules.ArrayFormat == nil {
		t.Fatal("ArrayFormat should not be nil")
	}
	if !rules.ArrayFormat.Enabled {
		t.Error("Enabled should be true")
	}
	if rules.ArrayFormat.MultilineThreshold != 3 {
		t.Errorf("MultilineThreshold: got %d, want 3", rules.ArrayFormat.MultilineThreshold)
	}
	if !rules.ArrayFormat.Sort {
		t.Error("Sort should be true")
	}
}

func TestParseHCLRequiredFields(t *testing.T) {
	rules := loadTestRules(t, `rules {
  required_fields {
    include {
      expose = true
    }
  }
}`)

	if rules.RequiredFields == nil || rules.RequiredFields.Include == nil {
		t.Fatal("RequiredFields.Include should not be nil")
	}
	if !rules.RequiredFields.Include.Expose {
		t.Error("Include.Expose should be true")
	}
}

func TestParseHCLCountForEach(t *testing.T) {
	rules := loadTestRules(t, `rules {
  count_for_each {
    enabled                = true
    warn_on_count_zero     = true
    warn_on_empty_for_each = true
    warn_on_conflict       = true
  }
}`)

	if rules.CountForEach == nil {
		t.Fatal("CountForEach should not be nil")
	}
	if !rules.CountForEach.Enabled ||
		!rules.CountForEach.WarnOnCountZero ||
		!rules.CountForEach.WarnOnEmptyForEach ||
		!rules.CountForEach.WarnOnConflict {
		t.Errorf("expected all flags true, got %+v", rules.CountForEach)
	}
}

func TestParseHCLDependencyOutputs(t *testing.T) {
	rules := loadTestRules(t, `rules {
  dependency_outputs {
    enabled = true
  }
}`)

	if rules.DependencyOutputs == nil {
		t.Fatal("DependencyOutputs should not be nil")
	}
	if !rules.DependencyOutputs.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestParseValuePatternAttribute(t *testing.T) {
	rules := loadTestRules(t, `rules {
  key_value {
    enabled = true
    value_pattern = {
      region = "^eu-"
      env    = "^(dev|prod)$"
    }
  }
}`)

	if rules.KeyValue == nil {
		t.Fatal("KeyValue should not be nil")
	}
	if got := rules.KeyValue.ValuePattern["region"]; got != "^eu-" {
		t.Errorf("region pattern: got %q, want ^eu-", got)
	}
	if got := rules.KeyValue.ValuePattern["env"]; got != "^(dev|prod)$" {
		t.Errorf("env pattern: got %q, want ^(dev|prod)$", got)
	}
}

func TestParseNestedOrderAttribute(t *testing.T) {
	rules := loadTestRules(t, `rules {
  block_order {
    enabled = true
    order   = ["terraform"]
    nested_order = {
      terraform = ["before_hook", "after_hook"]
    }
  }
}`)

	if rules.BlockOrder == nil {
		t.Fatal("BlockOrder should not be nil")
	}
	nested := rules.BlockOrder.NestedOrder["terraform"]
	if len(nested) != 2 || nested[0] != "before_hook" || nested[1] != "after_hook" {
		t.Errorf("nested_order.terraform: got %v, want [before_hook after_hook]", nested)
	}
}

func TestLoadConfigDir_Empty(t *testing.T) {
	t.Setenv("HCL_LINTER_CONFIG_DIR", "")
	t.Chdir(t.TempDir())

	_, err := LoadConfigDir("")
	if err == nil {
		t.Error("expected error for empty dir with no discovery source")
	}
}

func TestLoadConfigDir_Explicit(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestConfig(t, tmpDir, `rules {}`)

	loader, err := LoadConfigDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigDir: %v", err)
	}
	if loader == nil {
		t.Fatal("expected loader, got nil")
	}
}

func TestLoadConfigDir_NonExistent(t *testing.T) {
	_, err := LoadConfigDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Error("expected error for non-existent dir")
	}
}

func TestLoadConfigDirWithResult(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	loader, result := LoadConfigDirWithResult(tmpDir)
	if loader == nil {
		t.Fatal("expected loader")
	}
	if result.Source != ConfigSourceCLI {
		t.Errorf("got source %v, want ConfigSourceCLI", result.Source)
	}
	if result.SourcePath != configDir {
		t.Errorf("got path %s, want %s", result.SourcePath, configDir)
	}
}

func TestExtendsChain(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "base.hcl"), []byte(`rules {
  duplicates {
    enabled = true
    blocks  = ["dependency"]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	writeTestConfig(t, tmpDir, `extends = "base"
rules {
  block_order {
    enabled = true
    order   = ["include", "terraform"]
  }
}`)

	rules, err := NewLoader(tmpDir).LoadForFile("terragrunt.hcl")
	if err != nil {
		t.Fatalf("LoadForFile: %v", err)
	}

	if rules.BlockOrder == nil || !rules.BlockOrder.Enabled {
		t.Error("child block_order not applied")
	}
	if rules.Duplicates == nil || !rules.Duplicates.Enabled {
		t.Error("inherited duplicates not applied")
	}
}

func TestExtendsCircular(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "a.hcl"), []byte(`extends = "b"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.hcl"), []byte(`extends = "a"`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestConfig(t, tmpDir, `extends = "a"`)

	_, err := NewLoader(tmpDir).LoadForFile("terragrunt.hcl")
	if err == nil {
		t.Error("expected circular-extends error, got nil")
	}
}
