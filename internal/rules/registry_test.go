package rules_test

import (
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

// allRegisteredRules mirrors engine.New() so we can assert Rule.Name on every
// rule without creating a dependency cycle through the engine package.
func allRegisteredRules() []rules.Rule {
	return []rules.Rule{
		rules.BlockOrderRule{},
		rules.ArrayFormatRule{},
		rules.NameValidationRule{},
		rules.DuplicatesRule{},
		rules.RequiredFieldsRule{},
		rules.RequiredBlocksRule{},
		rules.BlankLinesRule{},
		rules.DependencyPathsRule{},
		rules.IncludePathsRule{},
		rules.RemoteStateRule{},
		rules.HCLFunctionsRule{},
		rules.TerraformBlockRule{},
		rules.KeyValueRule{},
		rules.CountForEachRule{},
		rules.DependencyOutputsRule{},
	}
}

// TestRuleNamesAreUnique verifies that every rule reports a unique,
// non-empty Name(). The Name is part of the public Rule interface but isn't
// read on the hot path, so we pin it here.
func TestRuleNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range allRegisteredRules() {
		name := r.Name()
		if name == "" {
			t.Errorf("%T returned empty Name()", r)
		}
		if seen[name] {
			t.Errorf("duplicate rule name %q (from %T)", name, r)
		}
		seen[name] = true
	}
}

// TestRuleEnabledOnNilCfg pins that every rule's Enabled() returns false when
// the config is nil - i.e. rules don't panic or unconditionally enable
// themselves when there's no user config.
func TestRuleEnabledOnNilCfg(t *testing.T) {
	for _, r := range allRegisteredRules() {
		if r.Enabled(nil) {
			t.Errorf("%T reported Enabled on nil config", r)
		}
	}
}

// TestRuleEnabledOnEmptyRules pins that every rule's Enabled() returns false
// when the Rules struct is zero-valued - protects against a rule that treats
// "nil sub-config" as "enabled by default".
func TestRuleEnabledOnEmptyRules(t *testing.T) {
	for _, r := range allRegisteredRules() {
		if r.Enabled(&config.Rules{}) {
			t.Errorf("%T reported Enabled on empty rules", r)
		}
	}
}

func TestRegistryRegisterAndAll(t *testing.T) {
	reg := &rules.Registry{}
	for _, r := range allRegisteredRules() {
		reg.Register(r)
	}

	all := reg.All()
	if len(all) != len(allRegisteredRules()) {
		t.Errorf("Registry.All() returned %d rules, want %d", len(all), len(allRegisteredRules()))
	}
}

func TestRegistryEnabledFiltersByConfig(t *testing.T) {
	reg := &rules.Registry{}
	for _, r := range allRegisteredRules() {
		reg.Register(r)
	}

	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{Enabled: true, Order: []string{"a"}},
	}
	active := reg.Enabled(cfg)
	if len(active) != 1 {
		t.Fatalf("expected 1 enabled rule, got %d: %+v", len(active), active)
	}
	if active[0].Name() != "block_order" {
		t.Errorf("got rule %q, want block_order", active[0].Name())
	}
}
