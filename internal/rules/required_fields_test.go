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
	n, err := rule.Fix(ctx)
	if err != nil {
		t.Fatalf("Fix error: %v", err)
	}
	if n == 0 {
		t.Fatal("expected Fix to report a change")
	}
	if !strings.Contains(string(ctx.Content), "expose = true") {
		t.Errorf("expected expose = true in output:\n%s", string(ctx.Content))
	}
}
