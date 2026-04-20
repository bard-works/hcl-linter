package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestRemoteStateBackendRequired(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		RemoteState: &config.RemoteStateConfig{Enabled: true, RequireBackend: true},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name: "remote_state with backend",
			content: `terraform {
  source = "./module"
  remote_state {
    backend = "s3"
    config { bucket = "my-bucket" }
  }
}
`,
			expectIssue: false,
		},
		{
			name: "remote_state without backend",
			content: `terraform {
  source = "./module"
  remote_state {
    config { bucket = "my-bucket" }
  }
}
`,
			expectIssue: true,
		},
		{
			name:        "terraform without remote_state",
			content:     `terraform { source = "./module" }` + "\n",
			expectIssue: false,
		},
		{
			name:        "empty remote_state block",
			content:     "terraform {\n  source = \"./module\"\n  remote_state {}\n}\n",
			expectIssue: true,
		},
	}

	r := rules.RemoteStateRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, "terragrunt.hcl")
			writeFile(t, file, tt.content)
			ctx := buildContextFromFile(t, file, cfg)
			issues := r.Check(ctx)

			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "remote_state_backend_required" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Errorf("expected remote_state_backend_required issue, got %v", issues)
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected remote_state_backend_required issue: %v", issues)
			}
		})
	}
}

func TestRemoteStateBackendEmptyString(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Rules{
		RemoteState: &config.RemoteStateConfig{Enabled: true, RequireBackend: true},
	}

	content := `terraform {
  source = "./module"
  remote_state {
    backend = ""
    config { bucket = "my-bucket" }
  }
}
`
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.RemoteStateRule{}
	ctx := buildContextFromFile(t, file, cfg)

	hasIssue := false
	for _, issue := range r.Check(ctx) {
		if issue.Rule == "remote_state_backend_required" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected remote_state_backend_required issue for empty backend string")
	}
}

func TestRemoteStateDisabled(t *testing.T) {
	cfg := &config.Rules{
		RemoteState: &config.RemoteStateConfig{Enabled: true, RequireBackend: false},
	}

	tmpDir := t.TempDir()
	content := "terraform {\n  source = \"./m\"\n  remote_state {}\n}\n"
	file := filepath.Join(tmpDir, "terragrunt.hcl")
	writeFile(t, file, content)

	r := rules.RemoteStateRule{}
	ctx := buildContextFromFile(t, file, cfg)
	if issues := r.Check(ctx); len(issues) != 0 {
		t.Errorf("expected no issues when RequireBackend is false, got %v", issues)
	}
}
