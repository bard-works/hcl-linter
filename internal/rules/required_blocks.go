package rules

import (
	"github.com/hashicorp/hcl/v2"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type RequiredBlocksRule struct{}

func (r RequiredBlocksRule) Name() string  { return "required_blocks" }
func (r RequiredBlocksRule) Priority() int { return PrioritySemantic }

func init() { Register(RequiredBlocksRule{}) }

func (r RequiredBlocksRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.RequiredBlocks != nil && len(cfg.RequiredBlocks.Required) > 0
}

func (r RequiredBlocksRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	blockCounts := make(map[string]int)
	for _, block := range ctx.Blocks {
		blockCounts[block.Type]++
	}
	for _, req := range ctx.Config.RequiredBlocks.Required {
		if req.Count == "once" && blockCounts[req.Type] != 1 {
			issues = append(issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "required_blocks",
				Message:  req.Error,
				Location: hcl.Range{
					Filename: ctx.FilePath,
					Start:    hcl.Pos{Line: 1, Column: 1},
					End:      hcl.Pos{Line: 1, Column: 1},
				},
			})
		}
	}
	return issues
}
