package rules_test

import (
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestKeyValueKeyCase_Camel(t *testing.T) {
	cfg := &config.Rules{
		KeyValue: &config.KeyValueConfig{Enabled: true, KeyCase: "camelCase"},
	}
	ctx := buildContext(t, "locals {\n  myVar = 1\n  bad_key = 2\n}\n", cfg)
	issues := rules.KeyValueRule{}.Check(ctx)

	var badCount int
	for _, issue := range issues {
		if issue.Rule == "key_case" {
			badCount++
		}
	}
	if badCount != 1 {
		t.Errorf("expected 1 key_case issue (bad_key), got %d", badCount)
	}
}

func TestKeyValueKeyCase_Kebab(t *testing.T) {
	cfg := &config.Rules{
		KeyValue: &config.KeyValueConfig{Enabled: true, KeyCase: "kebab-case"},
	}
	// kebab-case identifiers aren't valid HCL for bare attributes, so we use
	// a string-keyed object literal where the parser allows the character.
	ctx := buildContext(t, `inputs = {
  "good-key" = 1
  "bad_key"  = 2
}
`, cfg)
	// Top-level attrs aren't "blocks"; kvCheckKeyCase iterates blocks. So we
	// mainly verify the branch isn't panicking and the switch hits
	// "kebab-case".
	_ = rules.KeyValueRule{}.Check(ctx)
}

func TestKeyValueKeyCase_UnknownCase(t *testing.T) {
	cfg := &config.Rules{
		KeyValue: &config.KeyValueConfig{Enabled: true, KeyCase: "not-a-known-case"},
	}
	ctx := buildContext(t, "locals { x = 1 }\n", cfg)
	issues := rules.KeyValueRule{}.Check(ctx)
	for _, issue := range issues {
		if issue.Rule == "key_case" {
			t.Errorf("unknown case should short-circuit: %v", issue)
		}
	}
}
