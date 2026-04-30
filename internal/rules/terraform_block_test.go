package rules

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

func buildContext(t *testing.T, content string, cfg *config.Rules) *Context {
	t.Helper()
	p := hclparse.NewParser()
	file, diags := p.ParseHCL([]byte(content), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse error: %s", diags.Error())
	}
	return &Context{
		FilePath: "test.hcl",
		Content:  []byte(content),
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   cfg,
	}
}

func TestTerraformVersionValid(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"v1.0.0", true},
		{"0.12.30", true},
		{"v0.0.0", true},
		{"10.20.30", true},
		{"1.0", true},
		{"v1.0", true},
		{"invalid", false},
		{"", false},
		{"latest", false},
		{"1.x", false},
		{"v", false},
		{"1.0.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := tfVersionValid(tt.version)
			if result != tt.valid {
				t.Errorf("tfVersionValid(%q) = %v, want %v", tt.version, result, tt.valid)
			}
		})
	}
}

func TestTerraformConstraintValid(t *testing.T) {
	tests := []struct {
		constraint string
		valid      bool
	}{
		{">=1.0.0", true},
		{"<=2.0.0", true},
		{">1.0.0", true},
		{"<2.0.0", true},
		{"~>1.0.0", true},
		{"!=1.0.0", true},
		{"==1.0.0", true},
		{">= 1.0.0", true},
		{">=1.0.0, <2.0.0", true},
		{">=1.0.0,<2.0.0", true},
		{"1.0.0", true},
		{"v1.0.0", true},
		{"1.x", false},
		{"invalid", false},
		{"", false},
		{"latest", false},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			result := tfConstraintValid(tt.constraint)
			if result != tt.valid {
				t.Errorf("tfConstraintValid(%q) = %v, want %v", tt.constraint, result, tt.valid)
			}
		})
	}
}

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

	r := TerraformBlockRule{}
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

	r := TerraformBlockRule{}
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

	r := TerraformBlockRule{}
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

	r := TerraformBlockRule{}
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
