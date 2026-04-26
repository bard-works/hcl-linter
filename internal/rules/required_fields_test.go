package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestRequiredFieldsRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		RequiredFields: &config.RequiredFieldsConfig{
			Include: &config.IncludeRequired{Expose: true},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "include with expose",
			content: `include "root" {
  expose = true
}
`,
			expectIssue: false,
		},
		{
			name: "include without expose",
			content: `include "root" {
  path = "..."
}
`,
			expectIssue: true,
		},
		{
			name: "include without expose multiline",
			content: `include "root" {
  path = "some/path"
}
`,
			expectIssue: true,
		},
	}

	rule := rules.RequiredFieldsRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "required_fields" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected required_fields issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected required_fields issue: %v", issues)
			}
		})
	}
}

func TestRequiredFieldsRuleFix(t *testing.T) {
	cfg := &config.Rules{
		RequiredFields: &config.RequiredFieldsConfig{
			Include: &config.IncludeRequired{Expose: true},
		},
	}

	content := `include "root" {
  path = "some/path"
}
`
	ctx := buildContext(t, content, cfg)
	rule := rules.RequiredFieldsRule{}
	out, changed, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix error: %v", err)
	}
	if !changed {
		t.Fatal("expected Fix to report a change")
	}
	if !strings.Contains(string(out), "expose = true") {
		t.Errorf("expected expose = true in output:\n%s", string(out))
	}
}

func TestRequiredBlocksRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		RequiredBlocks: &config.RequiredBlocksConfig{
			Required: []config.RequiredBlockSpec{
				{Type: "terraform", Count: "once", Error: "missing terraform block"},
			},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
		issueMsg    string
	}{
		{
			name: "has terraform block",
			content: `terraform {}
`,
			expectIssue: false,
		},
		{
			name: "missing terraform block",
			content: `include "root" {
  path = "..."
}
`,
			expectIssue: true,
			issueMsg:    "missing terraform block",
		},
	}

	rule := rules.RequiredBlocksRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "required_blocks" {
					if tt.issueMsg == "" || issue.Message == tt.issueMsg {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected required_blocks issue %q, got %v", tt.issueMsg, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected required_blocks issue: %v", issues)
			}
		})
	}
}
