package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestArrayFormatRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		ArrayFormat: &config.ArrayFormatConfig{Enabled: true},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name:        "multiline array - no issue",
			content:     "arr = [\n  \"a\",\n  \"b\",\n]\n",
			expectIssue: false,
		},
		{
			name:        "single-item inline - no issue",
			content:     "arr = [\"only\"]\n",
			expectIssue: false,
		},
		{
			name:        "two-item inline tuple inside block - issue",
			content:     "inputs {\n  arr = [\"a\", \"b\"]\n}\n",
			expectIssue: true,
		},
	}

	rule := rules.ArrayFormatRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "array_format" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected array_format issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected array_format issue: %v", issues)
			}
		})
	}
}

func TestArrayFormatRuleFixSort(t *testing.T) {
	tests := []struct {
		name     string
		sort     bool
		input    string
		wantSort bool
	}{
		{
			name:     "sort disabled - preserves order",
			sort:     false,
			input:    `arr = ["charlie", "alpha", "bravo"]` + "\n",
			wantSort: false,
		},
		{
			name:     "sort enabled - alphabetical",
			sort:     true,
			input:    `arr = ["charlie", "alpha", "bravo"]` + "\n",
			wantSort: true,
		},
	}

	rule := rules.ArrayFormatRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Rules{
				ArrayFormat: &config.ArrayFormatConfig{Enabled: true, Sort: tt.sort},
			}
			ctx := buildContext(t, tt.input, cfg)
			n, err := rule.Fix(ctx)
			if err != nil {
				t.Fatalf("Fix error: %v", err)
			}
			if n == 0 {
				t.Fatal("expected Fix to report a change")
			}
			got := string(ctx.Content)
			alphaIdx := strings.Index(got, "alpha")
			charlieIdx := strings.Index(got, "charlie")
			if tt.wantSort && alphaIdx > charlieIdx {
				t.Errorf("expected sorted output (alpha before charlie):\n%s", got)
			}
			if !tt.wantSort && charlieIdx > alphaIdx {
				t.Errorf("expected unsorted output (charlie before alpha):\n%s", got)
			}
		})
	}
}
