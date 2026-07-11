package rules_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func buildContext(t *testing.T, content string, cfg *config.Rules) *rules.Context {
	t.Helper()
	p := hclparse.NewParser()
	file, diags := p.ParseHCL([]byte(content), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse error: %s", diags.Error())
	}
	return &rules.Context{
		FilePath: "test.hcl",
		Content:  []byte(content),
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   cfg,
	}
}

func TestBlockOrderRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"include", "locals", "terraform"},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "correct order",
			content: `include "root" {}
locals {}
terraform {}
`,
			expectIssue: false,
		},
		{
			name: "wrong order - terraform before locals",
			content: `include "root" {}
terraform {}
locals {}
`,
			expectIssue: true,
		},
		{
			name: "wrong order - locals before include",
			content: `locals {}
include "root" {}
terraform {}
`,
			expectIssue: true,
		},
	}

	rule := rules.BlockOrderRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "block_order" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected block_order issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected block_order issue: %v", issues)
			}
		})
	}
}

func TestBlockOrderRuleNestedCheck(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform"},
			NestedOrder: map[string][]string{
				"terraform": {"before_hooks", "after_hooks"},
			},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "correct nested order",
			content: `terraform {
  before_hooks {
    command = "echo before"
  }
  after_hooks {
    command = "echo after"
  }
}
`,
			expectIssue: false,
		},
		{
			name: "wrong nested order - after before before",
			content: `terraform {
  after_hooks {
    command = "echo after"
  }
  before_hooks {
    command = "echo before"
  }
}
`,
			expectIssue: true,
		},
	}

	rule := rules.BlockOrderRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "block_order" && strings.Contains(issue.Message, "nested block") {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected nested block_order issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected nested block_order issue: %v", issues)
			}
		})
	}
}

func TestBlockOrderNestedSingleBlock(t *testing.T) {
	// Only one nested block → len(nestedBlocks) < 2 → no order check performed
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform"},
			NestedOrder: map[string][]string{
				"terraform": {"before_hooks", "after_hooks"},
			},
		},
	}
	content := `terraform {
  before_hooks {
    command = "echo before"
  }
}
`
	ctx := buildContext(t, content, cfg)
	issues := (rules.BlockOrderRule{}).Check(ctx)
	for _, issue := range issues {
		if issue.Rule == "block_order" && strings.Contains(issue.Message, "nested block") {
			t.Errorf("unexpected nested block_order issue for single nested block: %s", issue.Message)
		}
	}
}

func TestFixBlockOrderNoBlocks(t *testing.T) {
	// Content with no blocks: FixBlockOrder should return unchanged content
	cfg := &config.BlockOrderConfig{
		Enabled: true,
		Order:   []string{"include", "locals", "terraform"},
	}
	content := "# just a comment\n"
	result := rules.FixBlockOrder(content, nil, cfg)
	// Result may differ in whitespace; key check: no panic, returns a string
	if result == "" {
		t.Error("expected non-empty result from FixBlockOrder with no blocks")
	}
}

func TestBlockOrderRuleFix(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"include", "locals", "terraform"},
		},
	}

	content := "terraform {}\nlocals {}\ninclude \"root\" {}\n"
	ctx := buildContext(t, content, cfg)

	rule := rules.BlockOrderRule{}
	n, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if n == 0 {
		t.Fatal("expected Fix to report a change")
	}
	got := string(ctx.Content)
	includePos := strings.Index(got, "include")
	localsPos := strings.Index(got, "locals")
	terraformPos := strings.Index(got, "terraform")
	if includePos >= localsPos || localsPos >= terraformPos {
		t.Errorf("blocks not reordered correctly:\n%s", got)
	}
}

func TestFixBlockOrderCommentAttachment(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform", "dependency"},
		},
	}
	content := `# vpc dependency
dependency "vpc" {
  config_path = "../vpc"
}

terraform {
  source = "..."
}
`
	ctx := buildContext(t, content, cfg)
	rule := rules.BlockOrderRule{}
	n, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if n == 0 {
		t.Fatal("expected Fix to report a change")
	}
	got := string(ctx.Content)
	commentPos := strings.Index(got, "# vpc dependency")
	dependencyPos := strings.Index(got, `dependency "vpc"`)
	terraformPos := strings.Index(got, "terraform {")
	if commentPos == -1 || dependencyPos == -1 || terraformPos == -1 {
		t.Fatalf("expected markers present, got:\n%s", got)
	}
	if !strings.Contains(got, "# vpc dependency\ndependency \"vpc\" {") {
		t.Errorf("expected comment to stay attached directly above its block, got:\n%s", got)
	}
	if terraformPos >= commentPos {
		t.Errorf("expected terraform block (and its comment+block unit) to move before it, got:\n%s", got)
	}
}

func TestFixBlockOrderBlankSeparatedCommentStays(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform", "dependency"},
		},
	}
	content := `# file header

dependency "vpc" {
  config_path = "../vpc"
}

terraform {
  source = "..."
}
`
	ctx := buildContext(t, content, cfg)
	rule := rules.BlockOrderRule{}
	if _, err := rule.Fix(ctx); err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	got := string(ctx.Content)
	if !strings.HasPrefix(got, "# file header\n") {
		t.Errorf("expected blank-separated header to remain at top, got:\n%s", got)
	}
}

func TestFixBlockOrderAttributePosition(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform", "include"},
		},
	}
	content := `include "root" {
  path = "../root.hcl"
}

inputs = {
  environment = "prod"
}

terraform {
  source = "..."
}
`
	ctx := buildContext(t, content, cfg)
	rule := rules.BlockOrderRule{}
	n, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if n == 0 {
		t.Fatal("expected Fix to report a change")
	}
	got := string(ctx.Content)
	terraformPos := strings.Index(got, "terraform {")
	inputsPos := strings.Index(got, "inputs = {")
	includePos := strings.Index(got, `include "root"`)
	if terraformPos == -1 || inputsPos == -1 || includePos == -1 {
		t.Fatalf("expected markers present, got:\n%s", got)
	}
	if terraformPos >= inputsPos || inputsPos >= includePos {
		t.Errorf("expected 'inputs' to stay interleaved between the two blocks, got:\n%s", got)
	}
}

func TestFixBlockOrderMultilineCommentAttachesAsUnit(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform", "dependency"},
		},
	}
	content := `/* multi
line */
dependency "vpc" {
  config_path = "../vpc"
}

terraform {
  source = "..."
}
`
	ctx := buildContext(t, content, cfg)
	rule := rules.BlockOrderRule{}
	n, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if n == 0 {
		t.Fatal("expected Fix to report a change")
	}
	got := string(ctx.Content)
	if !strings.Contains(got, "/* multi\nline */\ndependency \"vpc\" {") {
		t.Errorf("expected multi-line comment to attach as a unit, got:\n%s", got)
	}
	if strings.Index(got, "terraform {") >= strings.Index(got, "/* multi") {
		t.Errorf("expected terraform block to move before the comment+block unit, got:\n%s", got)
	}
}

func TestFixBlockOrderIdempotent(t *testing.T) {
	cfg := &config.BlockOrderConfig{
		Enabled: true,
		Order:   []string{"terraform", "dependency"},
	}
	content := `# vpc dependency
dependency "vpc" {
  config_path = "../vpc"
}

inputs = {
  environment = "prod"
}

terraform {
  source = "..."
}
`
	file, diags := hclparse.NewParser().ParseHCL([]byte(content), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse error: %s", diags.Error())
	}
	once := rules.FixBlockOrder(content, ast.GetTopLevelBlocks(file), cfg)

	// A fresh parser is required here: hclparse.Parser caches parsed files by
	// filename, so reusing the same parser+filename for a different content
	// string would silently return the first parse's stale AST.
	file2, diags := hclparse.NewParser().ParseHCL([]byte(once), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse error on fixed output: %s", diags.Error())
	}
	twice := rules.FixBlockOrder(once, ast.GetTopLevelBlocks(file2), cfg)

	if once != twice {
		t.Errorf("expected idempotent fix, first:\n%s\nsecond:\n%s", once, twice)
	}
}

func TestFixBlockOrderOutputReparses(t *testing.T) {
	cfg := &config.Rules{
		BlockOrder: &config.BlockOrderConfig{
			Enabled: true,
			Order:   []string{"terraform", "include", "dependency"},
		},
	}
	content := `# vpc dependency
dependency "vpc" {
  config_path = "../vpc"
}

inputs = {
  environment = "prod"
}

/* multi
line */
include "root" {
  path = "../root.hcl"
}

terraform {
  source = "..."
}
`
	ctx := buildContext(t, content, cfg)
	if _, err := (rules.BlockOrderRule{}).Fix(ctx); err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	_, diags := hclsyntax.ParseConfig(ctx.Content, "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("fixed output failed to re-parse: %s\n%s", diags.Error(), string(ctx.Content))
	}
}
