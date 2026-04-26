package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestKeyValueKeyCaseRule(t *testing.T) {
	cfg := &config.Rules{
		KeyValue: &config.KeyValueConfig{Enabled: true, KeyCase: "snake_case"},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
		issueRule   string
	}{
		{
			name:        "valid snake_case",
			content:     "locals {\n  my_var = \"test\"\n}\n",
			expectIssue: false,
		},
		{
			name:        "invalid camelCase in snake_case config",
			content:     "locals {\n  myVar = \"test\"\n}\n",
			expectIssue: true,
			issueRule:   "key_case",
		},
		{
			name:        "invalid kebab-case in snake_case config",
			content:     "locals {\n  my-var = \"test\"\n}\n",
			expectIssue: true,
			issueRule:   "key_case",
		},
	}

	r := rules.KeyValueRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "key_case" {
					hasIssue = true
					break
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected key_case issue, got %v", issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected key_case issue: %v", issues)
			}
		})
	}
}

func TestKeyValueDisallowedKeysRule(t *testing.T) {
	cfg := &config.Rules{
		KeyValue: &config.KeyValueConfig{Enabled: true, Disallowed: []string{"secret", "password"}},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name:        "allowed keys",
			content:     "locals {\n  name = \"test\"\n}\n",
			expectIssue: false,
		},
		{
			name:        "disallowed secret key",
			content:     "locals {\n  secret = \"abc\"\n}\n",
			expectIssue: true,
		},
		{
			name:        "disallowed password key",
			content:     "locals {\n  password = \"123\"\n}\n",
			expectIssue: true,
		},
	}

	r := rules.KeyValueRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "disallowed_keys" {
					hasIssue = true
					break
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected disallowed_keys issue, got %v", issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected disallowed_keys issue: %v", issues)
			}
		})
	}
}

func TestKeyValueValuePatternRule(t *testing.T) {
	cfg := &config.Rules{
		KeyValue: &config.KeyValueConfig{
			Enabled:      true,
			ValuePattern: map[string]string{"region": `^us-[a-z]+-[0-9]+$`},
		},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:        "valid region format",
			content:     "locals {\n  region = \"us-east-1\"\n}\n",
			expectIssue: false,
		},
		{
			name:          "invalid region format",
			content:       "locals {\n  region = \"invalid\"\n}\n",
			expectIssue:   true,
			issueContains: "does not match pattern",
		},
	}

	r := rules.KeyValueRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "value_pattern" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
						break
					}
				}
			}

			if tt.expectIssue && !hasIssue {
				t.Errorf("expected value_pattern issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected value_pattern issue: %v", issues)
			}
		})
	}
}
