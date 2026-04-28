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

func TestDependencyPathsRule(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing-module")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyPaths: &config.DependencyPathsConfig{Enabled: true},
	}

	tests := []struct {
		name          string
		content       string
		expectIssue   bool
		issueContains string
	}{
		{
			name:        "dependency with existing path",
			content:     `dependency "vpc" { config_path = "existing-module" }` + "\n",
			expectIssue: false,
		},
		{
			name:          "dependency with non-existing path",
			content:       `dependency "vpc" { config_path = "non-existent-module" }` + "\n",
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name:          "dependency with parent path",
			content:       `dependency "vpc" { config_path = "../vpc" }` + "\n",
			expectIssue:   true,
			issueContains: "does not exist",
		},
		{
			name: "multiple dependencies - one missing",
			content: `dependency "vpc" { config_path = "existing-module" }
dependency "db" { config_path = "missing-db" }
`,
			expectIssue:   true,
			issueContains: "missing-db",
		},
	}

	r := rules.DependencyPathsRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "dependency_path_exists" {
					if tt.issueContains == "" || strings.Contains(issue.Message, tt.issueContains) {
						hasIssue = true
					}
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected dependency_path_exists issue containing %q, got %v", tt.issueContains, issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected dependency_path_exists issue: %v", issues)
			}
		})
	}
}

func TestDependencyPathsWithAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyPaths: &config.DependencyPathsConfig{Enabled: true},
	}

	content := fmt.Sprintf(`dependency "vpc" { config_path = "%s" }`+"\n", filepath.ToSlash(existingDir))
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.DependencyPathsRule{}
	ctx := buildContextFromFile(t, file, cfg)
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "dependency_path_exists" {
			t.Errorf("unexpected dependency_path_exists issue: %v", issue.Message)
		}
	}
}

func TestMultipleDependenciesWithMixedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	vpcDir := filepath.Join(tmpDir, "vpc")
	if err := os.MkdirAll(vpcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Rules{
		DependencyPaths: &config.DependencyPathsConfig{Enabled: true},
	}

	content := `dependency "vpc" { config_path = "vpc" }
dependency "db" { config_path = "non-existent-db" }
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.DependencyPathsRule{}
	ctx := buildContextFromFile(t, file, cfg)

	count := 0
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "dependency_path_exists" {
			count++
			if !strings.Contains(issue.Message, "non-existent-db") {
				t.Errorf("issue should mention non-existent-db, got: %v", issue.Message)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 issue for non-existent-db, got %d", count)
	}
}
