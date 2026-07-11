package rules

import (
	"fmt"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type BlockOrderRule struct{}

func (r BlockOrderRule) Name() string  { return "block_order" }
func (r BlockOrderRule) Priority() int { return PriorityStructure }

func init() { Register(BlockOrderRule{}) }

func (r BlockOrderRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Ensures top-level blocks appear in the configured order.",
		Severity:    "error",
		Fixable:     true,
		ConfigBlock: "block_order",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{Name: "order", Type: "[]string", Required: true, Doc: "Block types in the desired sequence"},
			{
				Name:     "nested_order",
				Type:     "map[string][]string",
				Required: false,
				Default:  "{}",
				Doc:      "Per-parent nested block ordering (e.g. terraform = [\"before_hook\", \"after_hook\"])",
			},
		},
		Example: Example{
			Violation: `terraform {}
include "root" {}`,
			Fixed: `include "root" {}
terraform {}`,
		},
	}
}

func (r BlockOrderRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.BlockOrder != nil && cfg.BlockOrder.Enabled
}

func (r BlockOrderRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	checkBlockOrderIssues(&issues, ctx.Blocks, ctx.Config.BlockOrder)
	return issues
}

func checkBlockOrderIssues(issues *[]diag.Issue, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) {
	orderMap := make(map[string]int)
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	expectedOrder := make([]string, len(cfg.Order))
	copy(expectedOrder, cfg.Order)

	lastPos := -1
	for _, block := range blocks {
		pos := len(cfg.Order)
		if idx, ok := orderMap[block.Type]; ok {
			pos = idx
		}
		if pos < lastPos && lastPos < len(cfg.Order) {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "block_order",
				Message:  fmt.Sprintf("block type %q appears out of order. Expected: %v", block.Type, expectedOrder),
				Location: block.Block.TypeRange,
			})
		}
		lastPos = pos
	}

	if len(cfg.NestedOrder) > 0 {
		checkNestedBlockOrderIssues(issues, blocks, cfg.NestedOrder)
	}
}

func checkNestedBlockOrderIssues(issues *[]diag.Issue, blocks []ast.BlockInfo, nestedOrder map[string][]string) {
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
				if orderMap[currType] < orderMap[prevType] {
					pairKey := currType + ":" + prevType
					if !reportedPairs[pairKey] {
						*issues = append(*issues, diag.Issue{
							Severity: diag.SeverityError,
							Rule:     "block_order",
							Message: fmt.Sprintf(
								"nested block %q should come before %q inside %q block",
								currType,
								prevType,
								block.Type,
							),
							Location: nestedBlocks[i].TypeRange,
						})
						reportedPairs[pairKey] = true
					}
				}
			}
		}
	}
}

func normalizeBlankLines(lines []string) []string {
	var result []string
	prevWasBlank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if !prevWasBlank {
				result = append(result, line)
				prevWasBlank = true
			}
		} else {
			result = append(result, line)
			prevWasBlank = false
		}
	}
	return result
}

func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
