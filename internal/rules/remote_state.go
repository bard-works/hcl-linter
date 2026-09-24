package rules

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

// RemoteStateRule validates `remote_state` blocks - top-level, as Terragrunt
// defines them, or nested inside `terraform` - specifically, that the
// `backend` attribute is set when `require_backend` is configured.
type RemoteStateRule struct{}

func (r RemoteStateRule) Name() string  { return "remote_state" }
func (r RemoteStateRule) Priority() int { return PrioritySemantic }

func init() { Register(RemoteStateRule{}) }

func (r RemoteStateRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Requires backend to be set on remote_state blocks.",
		Severity:    "error",
		Fixable:     false,
		ConfigBlock: "remote_state",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{
				Name:     "require_backend",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Require the backend attribute to be set",
			},
		},
		Example: Example{
			Violation: `remote_state {
  config = {
    bucket = "my-state"
  }
}`,
		},
	}
}

func (r RemoteStateRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.RemoteState != nil && cfg.RemoteState.Enabled
}

func (r RemoteStateRule) Check(ctx *Context) []diag.Issue {
	cfg := ctx.Config.RemoteState
	if !cfg.RequireBackend {
		return nil
	}

	file, ok := ctx.File.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	var issues []diag.Issue
	for _, rs := range remoteStateBlocks(file) {
		attrs := ast.GetBlockAttributes(rs.Body)
		backend, ok := attrs["backend"]
		if !ok || hclStringValue(backend) == "" {
			issues = append(issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "remote_state_backend_required",
				Message:  "remote_state block missing required 'backend' attribute",
				Location: rs.TypeRange,
			})
		}
	}
	return issues
}

// remoteStateBlocks returns every remote_state block in the file: top-level
// ones (where Terragrunt defines them, usually in root.hcl) and, for
// backwards compatibility, ones nested inside a top-level terraform block.
func remoteStateBlocks(file *hclsyntax.Body) []*hclsyntax.Block {
	var out []*hclsyntax.Block
	for _, block := range file.Blocks {
		switch block.Type {
		case "remote_state":
			out = append(out, block)
		case "terraform":
			for _, nested := range block.Body.Blocks {
				if nested.Type == "remote_state" {
					out = append(out, nested)
				}
			}
		}
	}
	return out
}
