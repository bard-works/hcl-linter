package rules

import (
	"math/big"

	"github.com/zclconf/go-cty/cty"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type CountForEachRule struct{}

func (r CountForEachRule) Name() string  { return "count_for_each" }
func (r CountForEachRule) Priority() int { return PrioritySemantic }

func init() { Register(CountForEachRule{}) }

func (r CountForEachRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.CountForEach != nil && cfg.CountForEach.Enabled
}

func (r CountForEachRule) Check(ctx *Context) []diag.Issue {
	cfg := ctx.Config.CountForEach
	var issues []diag.Issue

	if cfg.WarnOnCountZero || cfg.WarnOnEmptyForEach || cfg.WarnOnConflict {
		cfeCheckCountZero(&issues, ctx.Blocks, cfg.WarnOnCountZero)
		cfeCheckEmptyForEach(&issues, ctx.Blocks, cfg.WarnOnEmptyForEach)
		cfeCheckConflict(&issues, ctx.Blocks, cfg.WarnOnConflict)
	}

	return issues
}

func cfeCheckCountZero(issues *[]diag.Issue, blocks []ast.BlockInfo, enabled bool) {
	if !enabled {
		return
	}

	for _, block := range blocks {
		if block.Type != "resource" && block.Type != "data" && block.Type != "module" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		countAttr, ok := attrs["count"]
		if !ok {
			continue
		}

		val, diags := countAttr.Value(nil)
		if diags.HasErrors() {
			continue
		}

		if val.Type() == cty.Number {
			numVal := val.AsBigFloat()
			if numVal.Cmp(big.NewFloat(0)) == 0 {
				*issues = append(*issues, diag.Issue{
					Severity: diag.SeverityWarning,
					Rule:     "count_zero",
					Message:  block.Type + " block has count = 0, resource will not be created",
					Location: countAttr.Range(),
				})
			}
		}
	}
}

func cfeCheckEmptyForEach(issues *[]diag.Issue, blocks []ast.BlockInfo, enabled bool) {
	if !enabled {
		return
	}

	for _, block := range blocks {
		if block.Type != "resource" && block.Type != "data" && block.Type != "module" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		forEachAttr, ok := attrs["for_each"]
		if !ok {
			continue
		}

		val, diags := forEachAttr.Value(nil)
		if diags.HasErrors() {
			continue
		}

		valMap := val.AsValueMap()
		if len(valMap) == 0 {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityWarning,
				Rule:     "empty_for_each",
				Message:  block.Type + " block has empty for_each, resource will not be created",
				Location: forEachAttr.Range(),
			})
		}
	}
}

func cfeCheckConflict(issues *[]diag.Issue, blocks []ast.BlockInfo, enabled bool) {
	if !enabled {
		return
	}

	for _, block := range blocks {
		if block.Type != "resource" && block.Type != "data" && block.Type != "module" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		_, hasCount := attrs["count"]
		_, hasForEach := attrs["for_each"]

		if hasCount && hasForEach {
			countAttr := attrs["count"]
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "count_for_each_conflict",
				Message:  block.Type + " block has both count and for_each, which cannot be used together",
				Location: countAttr.Range(),
			})
		}
	}
}
