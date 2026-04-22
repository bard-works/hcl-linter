package rules

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

// RemoteStateRule validates `remote_state` blocks nested inside the top-level
// `terraform` block - specifically, that the `backend` attribute is set when
// `require_backend` is configured.
type RemoteStateRule struct{}

func (r RemoteStateRule) Name() string  { return "remote_state" }
func (r RemoteStateRule) Priority() int { return PrioritySemantic }

func init() { Register(RemoteStateRule{}) }

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
	for _, block := range file.Blocks {
		if block.Type != "terraform" {
			continue
		}
		var remoteStateBlock *hclsyntax.Block
		for _, nested := range block.Body.Blocks {
			if nested.Type == "remote_state" {
				remoteStateBlock = nested
				break
			}
		}
		if remoteStateBlock == nil {
			continue
		}

		attrs := ast.GetBlockAttributes(remoteStateBlock.Body)
		backend, ok := attrs["backend"]
		if !ok || hclStringValue(backend) == "" {
			issues = append(issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "remote_state_backend_required",
				Message:  "remote_state block missing required 'backend' attribute",
				Location: remoteStateBlock.TypeRange,
			})
		}
	}
	return issues
}
