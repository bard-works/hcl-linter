package rules_test

import (
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestDuplicatesRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		Duplicates: &config.DuplicatesConfig{
			Enabled: true,
			Blocks:  []string{"dependency", "include"},
		},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "no duplicates",
			content: `dependency "vpc" {}
dependency "db" {}
`,
			expectIssue: false,
		},
		{
			name: "duplicate dependency",
			content: `dependency "vpc" {}
dependency "vpc" {}
`,
			expectIssue: true,
		},
		{
			name: "duplicate include",
			content: `include "root" {}
include "root" {}
`,
			expectIssue: true,
		},
	}

	rule := rules.DuplicatesRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "duplicates" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected duplicates issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected duplicates issue: %v", issues)
			}
		})
	}
}

func TestDuplicatesBlocksFilter(t *testing.T) {
	cfg := &config.Rules{
		Duplicates: &config.DuplicatesConfig{
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
			name: "duplicate dependency - in filter - should flag",
			content: `dependency "vpc" {}
dependency "vpc" {}
`,
			expectIssue: true,
		},
		{
			name: "duplicate include - NOT in filter - should not flag",
			content: `include "root" {}
include "root" {}
`,
			expectIssue: false,
		},
		{
			name: "duplicate locals - NOT in filter - should not flag",
			content: `locals {}
locals {}
`,
			expectIssue: false,
		},
	}

	rule := rules.DuplicatesRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "duplicates" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected duplicates issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected duplicates issue for block outside filter: %v", issues)
			}
		})
	}
}

func TestDuplicatesNoBlocksFilter(t *testing.T) {
	cfg := &config.Rules{
		Duplicates: &config.DuplicatesConfig{Enabled: true},
	}

	content := `include "root" {}
include "root" {}
`
	rule := rules.DuplicatesRule{}
	ctx := buildContext(t, content, cfg)
	issues := rule.Check(ctx)

	hasIssue := false
	for _, issue := range issues {
		if issue.Rule == "duplicates" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected duplicates issue when no block filter is set, got none")
	}
}
