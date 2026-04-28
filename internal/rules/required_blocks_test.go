package rules_test

import (
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

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
