package rules

import (
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
	"github.com/hashicorp/hcl/v2"
)

type RequiredBlocksRule struct{}

func (r RequiredBlocksRule) Name() string { return "required_blocks" }

func (r RequiredBlocksRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.RequiredBlocks != nil && len(cfg.RequiredBlocks.Required) > 0
}

func (r RequiredBlocksRule) Check(ctx *Context) []linter.Issue {
	var issues []linter.Issue
	blockCounts := make(map[string]int)
	for _, block := range ctx.Blocks {
		blockCounts[block.Type]++
	}
	for _, req := range ctx.Config.RequiredBlocks.Required {
		if req.Count == "once" && blockCounts[req.Type] != 1 {
			issues = append(issues, linter.Issue{
				Severity: linter.SeverityError,
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
