package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestCountForEachRule(t *testing.T) {
	cfg := &config.Rules{
		CountForEach: &config.CountForEachConfig{
			Enabled:              true,
			WarnOnCountZero:      true,
			WarnOnEmptyForEach:   true,
			WarnOnConflict:       true,
		},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueRule     string
		issueContains string
	}{
		{
			name:          "resource with count = 0",
			content:       "resource \"aws_instance\" \"test\" {\n  count = 0\n}\n",
			expectIssue:   true,
			issueRule:     "count_zero",
			issueContains: "count = 0",
		},
		{
			name:          "resource with empty for_each",
			content:       "resource \"aws_instance\" \"test\" {\n  for_each = {}\n}\n",
			expectIssue:   true,
			issueRule:     "empty_for_each",
			issueContains: "empty for_each",
		},
		{
			name:          "resource with count and for_each conflict",
			content:       "resource \"aws_instance\" \"test\" {\n  count    = 1\n  for_each = {}\n}\n",
			expectIssue:   true,
			issueRule:     "count_for_each_conflict",
			issueContains: "both count and for_each",
		},
		{
			name:        "valid resource with count = 1",
			content:     "resource \"aws_instance\" \"test\" {\n  count = 1\n}\n",
			expectIssue: false,
		},
	}

	r := rules.CountForEachRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if tt.issueRule == "" {
					hasIssue = len(issues) > 0
					break
				}
				if issue.Rule == tt.issueRule {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected %s issue containing %q, got %v", tt.issueRule, tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected issue: %v", issues)
			}
		})
	}
}
