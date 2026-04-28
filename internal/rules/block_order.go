package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type BlockOrderRule struct{}

func (r BlockOrderRule) Name() string { return "block_order" }

func (r BlockOrderRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.BlockOrder != nil && cfg.BlockOrder.Enabled
}

func (r BlockOrderRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	checkBlockOrderIssues(&issues, ctx.Blocks, ctx.Config.BlockOrder)
	return issues
}

func (r BlockOrderRule) Fix(ctx *Context) ([]byte, bool, error) {
	newContent := FixBlockOrder(string(ctx.Content), ctx.Blocks, ctx.Config.BlockOrder)
	if newContent == string(ctx.Content) {
		return ctx.Content, false, nil
	}
	return []byte(newContent), true, nil
}

// FixBlockOrder reorders blocks according to cfg.
func FixBlockOrder(content string, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) string {
	lines := strings.Split(content, "\n")

	orderMap := make(map[string]int)
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	type blockWithContent struct {
		info    ast.BlockInfo
		content []string
	}

	blockContents := make([]blockWithContent, len(blocks))
	for i, block := range blocks {
		var blockLines []string
		for l := block.StartLine; l <= block.EndLine && l < len(lines); l++ {
			blockLines = append(blockLines, lines[l])
		}
		blockContents[i] = blockWithContent{block, blockLines}
	}

	sort.SliceStable(blockContents, func(i, j int) bool {
		posI := orderMap[blockContents[i].info.Type]
		posJ := orderMap[blockContents[j].info.Type]
		if posI == 0 && blockContents[i].info.Type != cfg.Order[0] {
			posI = len(cfg.Order)
		}
		if posJ == 0 && blockContents[j].info.Type != cfg.Order[0] {
			posJ = len(cfg.Order)
		}
		return posI < posJ
	})

	usedLines := make(map[int]bool)
	for _, block := range blocks {
		for l := block.StartLine; l <= block.EndLine; l++ {
			usedLines[l] = true
		}
	}

	var nonBlockLines []string
	for i, line := range lines {
		if !usedLines[i] {
			nonBlockLines = append(nonBlockLines, line)
		}
	}
	nonBlockLines = trimTrailingEmptyLines(nonBlockLines)

	var resultLines []string
	for i, bc := range blockContents {
		resultLines = append(resultLines, bc.content...)
		if i < len(blockContents)-1 {
			resultLines = append(resultLines, "")
		}
	}

	if len(nonBlockLines) > 0 {
		if len(resultLines) > 0 && strings.TrimSpace(resultLines[len(resultLines)-1]) != "" {
			resultLines = append(resultLines, "")
		}
		resultLines = append(resultLines, nonBlockLines...)
	}

	resultLines = normalizeBlankLines(resultLines)

	for len(resultLines) > 0 && strings.TrimSpace(resultLines[len(resultLines)-1]) == "" {
		resultLines = resultLines[:len(resultLines)-1]
	}

	return strings.Join(resultLines, "\n") + "\n"
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
