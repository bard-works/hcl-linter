package rules_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

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
		{
			name:        "invalid dependency - in filter - should flag",
			content:     `dependency "my-vpc" {}` + "\n",
			expectIssue: true,
		},
		{
			name:        "invalid include - NOT in filter - should not flag",
			content:     `include "my-root" {}` + "\n",
			expectIssue: false,
		},
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
		{"1starts_digit", true}, // !isLowerLetter(name[0])
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
	// Test basic label fix + all reference types
	content := `dependency "my-vpc" {
  config_path = "../vpc"
}

locals {
  vpc_id = dependency.my-vpc.outputs.id
  vpc_cidr = dependency["my-vpc"].outputs.cidr
}
`
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
	if strings.Contains(got, "my-vpc") {
		t.Errorf("expected 'my-vpc' replaced everywhere, got:\n%s", got)
	}
	if !strings.Contains(got, "my_vpc") {
		t.Errorf("expected 'my_vpc' in result, got:\n%s", got)
	}
	// Check all reference types updated
	if strings.Contains(got, `dependency.my-vpc.`) {
		t.Errorf("dot notation reference not updated, got:\n%s", got)
	}
	if strings.Contains(got, `dependency["my-vpc"]`) {
		t.Errorf("index notation reference not updated, got:\n%s", got)
	}
}

func TestNameValidationFixStringValueImmunity(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}
	content := `dependency "my-vpc" {
  config_path = "../my-vpc"
}

inputs = {
  note = "my-vpc"
}
`
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
	if !strings.Contains(got, `dependency "my_vpc"`) {
		t.Errorf("expected label renamed, got:\n%s", got)
	}
	if !strings.Contains(got, `config_path = "../my-vpc"`) {
		t.Errorf("expected string value '../my-vpc' untouched, got:\n%s", got)
	}
	if !strings.Contains(got, `note = "my-vpc"`) {
		t.Errorf("expected string value 'my-vpc' untouched, got:\n%s", got)
	}
}

func TestNameValidationFixCommentImmunity(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}
	content := `# see dependency.my-vpc for details
dependency "my-vpc" {
  config_path = "../vpc"
}
`
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
	if !strings.Contains(got, "# see dependency.my-vpc for details") {
		t.Errorf("expected comment untouched, got:\n%s", got)
	}
}

func TestNameValidationFixPrefixImmunity(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}
	content := `dependency "net-x" {
  config_path = "../net-x"
}

dependency "network" {
  config_path = "../network"
}

locals {
  a = dependency.net-x.outputs.id
  b = dependency.network.outputs.id
  c = dependency.net-x-standby.outputs.id
}
`
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
	if !strings.Contains(got, `dependency "net_x"`) {
		t.Errorf("expected 'net-x' label renamed, got:\n%s", got)
	}
	if !strings.Contains(got, "dependency.net_x.outputs.id") {
		t.Errorf("expected 'net-x' reference renamed, got:\n%s", got)
	}
	if !strings.Contains(got, `dependency "network"`) {
		t.Errorf("expected 'network' label untouched, got:\n%s", got)
	}
	if !strings.Contains(got, "dependency.network.outputs.id") {
		t.Errorf("expected 'network' reference untouched by 'net-x' rename, got:\n%s", got)
	}
	// "net-x-standby" shares "net-x" as a literal prefix but is a distinct
	// identifier token; it must not be corrupted by the "net-x" rename.
	if !strings.Contains(got, "dependency.net-x-standby.outputs.id") {
		t.Errorf("expected 'net-x-standby' reference untouched by 'net-x' rename, got:\n%s", got)
	}
}

func TestNameValidationFixTypeScoping(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}
	content := `dependency "my-vpc" {
  config_path = "../vpc"
}

include "my-vpc" {
  path = "../root.hcl"
}
`
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
	if !strings.Contains(got, `dependency "my_vpc"`) {
		t.Errorf("expected 'dependency' label renamed, got:\n%s", got)
	}
	if !strings.Contains(got, `include "my-vpc"`) {
		t.Errorf("expected 'include' label untouched (out of blockSet), got:\n%s", got)
	}
}

func TestNameValidationFixOutputReparses(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Blocks:  []string{"dependency"},
		},
	}
	content := `dependency "my-vpc" {
  config_path = "../vpc"
}

locals {
  vpc_id = dependency.my-vpc.outputs.id
  vpc_cidr = dependency["my-vpc"].outputs.cidr
}
`
	ctx := buildContext(t, content, cfg)
	rule := rules.NameValidationRule{}
	if _, err := rule.Fix(ctx); err != nil {
		t.Fatalf("Fix error: %v", err)
	}
	_, diags := hclsyntax.ParseConfig(ctx.Content, "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("fixed output failed to re-parse: %s\n%s", diags.Error(), string(ctx.Content))
	}
}

func TestNameValidationFixSpacesInLabel(t *testing.T) {
	cfg := &config.Rules{
		NameValidation: &config.NameValidationConfig{
			Enabled: true,
			Pattern: `^[a-z][a-z0-9_]*$`,
			Blocks:  []string{"dependency"},
		},
	}
	// Label with hyphen and space - should become "db_primary"
	content := `dependency "db- primary" {
  config_path = "../database"
}
`
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
	if strings.Contains(got, "db- primary") {
		t.Errorf("expected space and hyphen removed, got:\n%s", got)
	}
	if !strings.Contains(got, "db_primary") {
		t.Errorf("expected cleaned label 'db_primary', got:\n%s", got)
	}
}
