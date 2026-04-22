package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestNameValidationRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Pattern: `^[a-z][a-z0-9_]*$`,
			Blocks:  []string{"include", "dependency"},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{name: "valid name with underscore", content: `include "vault_azuread" {}` + "\n", expectIssue: false},
		{name: "valid simple name", content: `include "vpc" {}` + "\n", expectIssue: false},
		{name: "invalid name with hyphen", content: `include "vault-azuread" {}` + "\n", expectIssue: true},
		{name: "invalid dependency name", content: `dependency "my-vpc" {}` + "\n", expectIssue: true},
	}

	rule := rules.NameValidationRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "name_validation" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected name_validation issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected name_validation issue: %v", issues)
			}
		})
	}
}

func TestNameValidationBlocksFilter(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{name: "invalid dependency - in filter - should flag", content: `dependency "my-vpc" {}` + "\n", expectIssue: true},
		{name: "invalid include - NOT in filter - should not flag", content: `include "my-root" {}` + "\n", expectIssue: false},
	}

	rule := rules.NameValidationRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "name_validation" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected name_validation issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected name_validation issue for block outside filter: %v", issues)
			}
		})
	}
}

func TestNameValidationCustomPattern(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Pattern: `^[a-z][a-z0-9-_]*$`,
			Blocks:  []string{"dependency"},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{name: "hyphen allowed by custom pattern", content: `dependency "my-vpc" {}` + "\n", expectIssue: false},
		{name: "uppercase rejected by custom pattern", content: `dependency "MyVpc" {}` + "\n", expectIssue: true},
		{name: "valid lowercase name", content: `dependency "my_vpc" {}` + "\n", expectIssue: false},
	}

	rule := rules.NameValidationRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "name_validation" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected name_validation issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected name_validation issue: %v", issues)
			}
		})
	}
}

func TestNameValidationNoFilter(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{Enabled: true},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{name: "invalid name on any block - should flag", content: `include "my-root" {}` + "\n", expectIssue: true},
		{name: "valid name - should not flag", content: `include "my_root" {}` + "\n", expectIssue: false},
	}

	rule := rules.NameValidationRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "name_validation" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected name_validation issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected name_validation issue: %v", issues)
			}
		})
	}
}

func TestNameValidationDefaultPatternEdgeCases(t *testing.T) {
	// No pattern set → uses isValidIdentifier; covers the empty/digit-start branches
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{Enabled: true},
	}
	tests := []struct {
		label       string
		expectIssue bool
	}{
		{"1starts_digit", true},  // !isLowerLetter(name[0])
		{"valid_name", false},
	}
	for _, tt := range tests {
		content := "include \"" + tt.label + "\" {}\n"
		ctx := buildContext(t, content, cfg)
		issues := rules.NameValidationRule{}.Check(ctx)
		found := false
		for _, i := range issues {
			if i.Rule == "name_validation" {
				found = true
				break
			}
		}
		if tt.expectIssue && !found {
			t.Errorf("label %q: expected issue, got none", tt.label)
		}
		if !tt.expectIssue && found {
			t.Errorf("label %q: unexpected issue", tt.label)
		}
	}
}

func TestNameValidationFixNoChange(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{Enabled: true, Blocks: []string{"dependency"}},
	}
	// Already valid → Fix should return 0
	ctx := buildContext(t, "dependency \"my_vpc\" {}\n", cfg)
	n, err := rules.NameValidationRule{}.Fix(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no changes, got %d", n)
	}
}

func TestNameValidationRuleFix(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}
	content := `dependency "my-vpc" {}` + "\n"
	ctx := buildContext(t, content, cfg)

	rule := rules.NameValidationRule{}
	n, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix error: %v", err)
	}
	if n == 0 {
		t.Fatal("expected Fix to report a change")
	}
	got := string(ctx.Content)
	if got == content {
		t.Errorf("content unchanged after fix:\n%s", got)
	}
	if !strings.Contains(got, "my_vpc") {
		t.Errorf("expected hyphen replaced with underscore, got:\n%s", got)
	}
}
