package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresetsParseAndValidate(t *testing.T) {
	names := PresetNames()
	if len(names) < 2 {
		t.Fatalf("expected terraform and terragrunt presets, got %v", names)
	}
	for _, name := range names {
		dir := t.TempDir()
		path := filepath.Join(dir, "default.hcl")
		if err := os.WriteFile(path, []byte(`extends = "builtin:`+name+`"`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rules, err := loadHCLConfig(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !rules.IsEnabled() {
			t.Errorf("%s: preset enables no rules", name)
		}
		if issues := ValidateConfigFile(path); len(issues) != 0 {
			t.Errorf("%s: validation issues: %v", name, issues)
		}
		src, _ := presetSource(name)
		if issues := detectUnknownBlocksInSource(t, src); issues != "" {
			t.Errorf("%s: %s", name, issues)
		}
	}
}

// detectUnknownBlocksInSource runs the unknown-block check over preset source
// so a typo in a shipped preset fails the build rather than silently no-oping.
func detectUnknownBlocksInSource(t *testing.T, src []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "preset.hcl")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, is := range detectUnknownBlocks(path) {
		msgs = append(msgs, is.Message)
	}
	return strings.Join(msgs, "; ")
}

func TestPresetChildOverridesRuleBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "terragrunt.hcl")
	src := `extends = "builtin:terragrunt"

rules {
  block_order {
    enabled = true
    order   = ["terraform", "include"]
  }
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := loadHCLConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := rules.BlockOrder.Order; len(got) != 2 || got[0] != "terraform" {
		t.Errorf("child block_order should replace preset's, got %v", got)
	}
	if rules.DependencyPaths == nil || !rules.DependencyPaths.Enabled {
		t.Error("rules not overridden by the child should be inherited from the preset")
	}
}

func TestUnknownPreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.hcl")
	if err := os.WriteFile(path, []byte(`extends = "builtin:nope"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadHCLConfig(path)
	if err == nil || !strings.Contains(err.Error(), "builtin:terragrunt") {
		t.Fatalf("expected unknown-preset error listing available presets, got %v", err)
	}
}

// Terraform allows several locals/terraform/provider blocks per module, so the
// terraform preset must not treat repeats of those as duplicates.
func TestTerraformPresetAllowsRepeatableBlocks(t *testing.T) {
	src, _ := presetSource("terraform")
	rules, err := parseHCLConfig(src, "builtin:terraform", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range rules.Duplicates.Blocks {
		if b == "locals" || b == "terraform" || b == "provider" {
			t.Errorf("terraform preset flags repeatable block %q as duplicate", b)
		}
	}
}
