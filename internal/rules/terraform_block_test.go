package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestTerraformSourceRequired(t *testing.T) {
	cfg := &config.Rules{
		TerraformBlock: &config.TerraformBlockConfig{Enabled: true, SourceRequired: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:        "terraform block with source",
			content:     "terraform {\n  source = \"./module\"\n}\n",
			expectIssue: false,
		},
		{
			name:          "terraform block without source",
			content:       "terraform {\n  version = \"1.0.0\"\n}\n",
			expectIssue:   true,
			issueContains: "must have 'source' attribute",
		},
		{
			name:        "no terraform block",
			content:     "locals {\n  name = \"test\"\n}\n",
			expectIssue: false,
		},
	}

	r := rules.TerraformBlockRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "terraform_source_required" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_source_required issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_source_required issue: %v", issues)
			}
		})
	}
}

func TestTerraformVersionFormat(t *testing.T) {
	cfg := &config.Rules{
		TerraformBlock: &config.TerraformBlockConfig{Enabled: true, VersionFormat: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{name: "valid version", content: "terraform {\n  version = \"1.0.0\"\n}\n", expectIssue: false},
		{name: "valid version with v prefix", content: "terraform {\n  version = \"v1.5.2\"\n}\n", expectIssue: false},
		{name: "invalid version format", content: "terraform {\n  version = \"latest\"\n}\n", expectIssue: true, issueContains: "may not match expected format"},
		{name: "valid required_version", content: "terraform {\n  required_version = \">= 1.0.0\"\n}\n", expectIssue: false},
		{name: "valid required_version with range", content: "terraform {\n  required_version = \">= 1.0.0, < 2.0.0\"\n}\n", expectIssue: false},
		{name: "invalid required_version", content: "terraform {\n  required_version = \"1.x\"\n}\n", expectIssue: true, issueContains: "may not match expected format"},
	}

	r := rules.TerraformBlockRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "terraform_version_format" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_version_format issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_version_format issue: %v", issues)
			}
		})
	}
}

func TestTerraformExtraArguments(t *testing.T) {
	cfg := &config.Rules{
		TerraformBlock: &config.TerraformBlockConfig{Enabled: true, ExtraArgumentsValid: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:          "extra_arguments with name but no args",
			content:       "terraform {\n  extra_arguments \"example\" {\n    name = \"example\"\n  }\n}\n",
			expectIssue:   true,
			issueContains: "should have 'arguments' or nested blocks",
		},
		{
			name:        "extra_arguments with arguments",
			content:     "terraform {\n  extra_arguments \"example\" {\n    arguments = [\"-var\", \"foo=bar\"]\n  }\n}\n",
			expectIssue: false,
		},
		{
			name:          "extra_arguments missing name",
			content:       "terraform {\n  extra_arguments {\n    arguments = [\"-var\", \"foo=bar\"]\n  }\n}\n",
			expectIssue:   true,
			issueContains: "should have a non-empty 'name' attribute",
		},
		{
			name:          "extra_arguments empty",
			content:       "terraform {\n  extra_arguments \"example\" {}\n}\n",
			expectIssue:   true,
			issueContains: "should have 'arguments' or nested blocks",
		},
	}

	r := rules.TerraformBlockRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "terraform_extra_arguments_valid" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_extra_arguments_valid issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_extra_arguments_valid issue: %v", issues)
			}
		})
	}
}

func TestTerraformDeprecatedFields(t *testing.T) {
	cfg := &config.Rules{
		TerraformBlock: &config.TerraformBlockConfig{Enabled: true, NoDeprecatedFields: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:          "deprecated terraform nested block",
			content:       "terraform {\n  terraform {\n    source = \"./module\"\n  }\n}\n",
			expectIssue:   true,
			issueContains: "block 'terraform' is deprecated",
		},
		{
			name:          "deprecated before_hook block",
			content:       "terraform {\n  before_hook {\n    commands = [\"echo hello\"]\n  }\n}\n",
			expectIssue:   true,
			issueContains: "block 'before_hook' is deprecated",
		},
		{
			name:          "deprecated after_hook block",
			content:       "terraform {\n  after_hook {\n    commands = [\"echo hello\"]\n  }\n}\n",
			expectIssue:   true,
			issueContains: "block 'after_hook' is deprecated",
		},
		{
			name:        "valid modern fields",
			content:     "terraform {\n  source = \"./module\"\n}\n",
			expectIssue: false,
		},
	}

	r := rules.TerraformBlockRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := r.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "terraform_deprecated_fields" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected terraform_deprecated_fields issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected terraform_deprecated_fields issue: %v", issues)
			}
		})
	}
}
