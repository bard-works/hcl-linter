package linter

import (
	"math/big"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/zclconf/go-cty/cty"
)

func checkCountForEachImpl(result *Result, blocks []ast.BlockInfo, cfg *config.CountForEachConfig) {
	if cfg.WarnOnCountZero || cfg.WarnOnEmptyForEach || cfg.WarnOnConflict {
		checkCountZero(result, blocks, cfg.WarnOnCountZero)
		checkEmptyForEach(result, blocks, cfg.WarnOnEmptyForEach)
		checkConflict(result, blocks, cfg.WarnOnConflict)
	}
}

func checkCountZero(result *Result, blocks []ast.BlockInfo, enabled bool) {
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
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityWarning,
					Rule:     "count_zero",
					Message:  block.Type + " block has count = 0, resource will not be created",
					Location: countAttr.Range(),
				})
			}
		}
	}
}

func checkEmptyForEach(result *Result, blocks []ast.BlockInfo, enabled bool) {
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
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "empty_for_each",
				Message:  block.Type + " block has empty for_each, resource will not be created",
				Location: forEachAttr.Range(),
			})
		}
	}
}

func checkConflict(result *Result, blocks []ast.BlockInfo, enabled bool) {
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
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "count_for_each_conflict",
				Message:  block.Type + " block has both count and for_each, which cannot be used together",
				Location: countAttr.Range(),
			})
		}
	}
}
