package linter

import (
	"fmt"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

func checkBlockOrderImpl(result *Result, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) {
	orderMap := make(map[string]int)
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	expectedOrder := make([]string, 0, len(cfg.Order))
	expectedOrder = append(expectedOrder, cfg.Order...)

	lastPos := -1
	for _, block := range blocks {
		pos := len(cfg.Order)
		if idx, ok := orderMap[block.Type]; ok {
			pos = idx
		}

		if pos < lastPos && lastPos < len(cfg.Order) {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "block_order",
				Message:  fmt.Sprintf("block type %q appears out of order. Expected: %v", block.Type, expectedOrder),
				Location: block.Block.TypeRange,
			})
		}
		lastPos = pos
	}

	if len(cfg.NestedOrder) > 0 {
		checkNestedBlockOrderImpl(result, blocks, cfg.NestedOrder)
	}
}

func checkNestedBlockOrderImpl(result *Result, blocks []ast.BlockInfo, nestedOrder map[string][]string) {
	for _, block := range blocks {
		order, ok := nestedOrder[block.Type]
		if !ok {
			continue
		}

		nestedBlocks := block.Block.Body.Blocks
		if len(nestedBlocks) < 2 {
			continue
		}

		orderMap := make(map[string]int)
		for i, name := range order {
			orderMap[name] = i
		}

		var orderedBlocks []string
		for _, nb := range nestedBlocks {
			if _, ok := orderMap[nb.Type]; ok {
				orderedBlocks = append(orderedBlocks, nb.Type)
			}
		}

		if len(orderedBlocks) > 1 {
			reportedPairs := make(map[string]bool)
			for i := 1; i < len(orderedBlocks); i++ {
				currType := orderedBlocks[i]
				prevType := orderedBlocks[i-1]

				currIdx := orderMap[currType]
				prevIdx := orderMap[prevType]

				if currIdx < prevIdx {
					pairKey := currType + ":" + prevType
					if !reportedPairs[pairKey] {
						result.Issues = append(result.Issues, Issue{
							Severity: SeverityError,
							Rule:     "block_order",
							Message:  fmt.Sprintf("nested block %q should come before %q inside %q block", currType, prevType, block.Type),
							Location: nestedBlocks[i].TypeRange,
						})
						reportedPairs[pairKey] = true
					}
				}
			}
		}
	}
}
