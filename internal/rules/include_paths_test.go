package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestIncludePathsRule(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing-parent")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		IncludePaths: &config.IncludePathsConfig{Enabled: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:        "include with existing path",
			content:     `include "root" { path = "existing-parent" }` + "\n",
			expectIssue: false,
		},
		{
			name:          "include with non-existing path",
			content:       `include "root" { path = "non-existent" }` + "\n",
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name:        "include with function call - skipped",
			content:     `include "root" { path = find_in_parent_folders() }` + "\n",
			expectIssue: false,
		},
	}

	r := rules.IncludePathsRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "include_path_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected include_path_exists issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected include_path_exists issue: %v", issues)
			}
		})
	}
}

func TestIncludePathsNoPathAttr(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Rules{
		IncludePaths: &config.IncludePathsConfig{Enabled: true},
	}

	// include block with no path attr → attrs["path"] lookup returns !ok → skipped
	content := `include "root" { expose = true }` + "\n"
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)
	ctx := buildContextFromFile(t, file, cfg)

	for _, issue := range (rules.IncludePathsRule{}).Check(ctx) {
		if issue.Rule == "include_path_exists" {
			t.Error("unexpected include_path_exists issue for include with no path attr")
		}
	}
}

func TestIncludePathsWithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		IncludePaths: &config.IncludePathsConfig{Enabled: true},
	}

	content := fmt.Sprintf(`include "root" { path = "%s" }`+"\n", filepath.ToSlash(existingDir))
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.IncludePathsRule{}
	ctx := buildContextFromFile(t, file, cfg)
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "include_path_exists" {
			t.Errorf("unexpected include_path_exists issue: %v", issue.Message)
		}
	}
}
