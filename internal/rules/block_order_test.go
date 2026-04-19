package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
	"github.com/hashicorp/hcl/v2/hclparse"
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
	out, changed, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if !changed {
		t.Fatal("expected Fix to report a change")
	}
	got := string(out)
	includePos := strings.Index(got, "include")
	localsPos := strings.Index(got, "locals")
	terraformPos := strings.Index(got, "terraform")
	if !(includePos < localsPos && localsPos < terraformPos) {
		t.Errorf("blocks not reordered correctly:\n%s", got)
	}
}
