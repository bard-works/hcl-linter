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
			Enabled:            true,
			WarnOnCountZero:    true,
			WarnOnEmptyForEach: true,
			WarnOnConflict:     true,
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

	// Disabled paths — when individual warn flags are false no issues should be emitted
	t.Run("count_zero disabled", func(t *testing.T) {
		disabledCfg := &config.Rules{
			CountForEach: &config.CountForEachConfig{
				Enabled:            true,
				WarnOnCountZero:    false,
				WarnOnEmptyForEach: false,
				WarnOnConflict:     false,
			},
		}
		ctx := buildContext(t, "resource \"aws_instance\" \"x\" {\n  count = 0\n  for_each = {}\n}\n", disabledCfg)
		if issues := (rules.CountForEachRule{}).Check(ctx); len(issues) != 0 {
			t.Errorf("expected no issues when all warn flags disabled, got %v", issues)
		}
	})

	// Non-resource block type should never trigger count/for_each checks
	t.Run("non-resource block ignored", func(t *testing.T) {
		ctx := buildContext(t, "locals {\n  count = 0\n}\n", cfg)
		issues := (rules.CountForEachRule{}).Check(ctx)
		for _, i := range issues {
			if i.Rule == "count_zero" {
				t.Error("unexpected count_zero on locals block")
			}
		}
	})

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
