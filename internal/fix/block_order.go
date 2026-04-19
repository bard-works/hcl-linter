package fix

import (
	"sort"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

// fixBlockOrder reorders blocks according to the given configuration.
func fixBlockOrder(content string, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) string {
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

// normalizeBlankLines removes consecutive blank lines, keeping at most one.
func normalizeBlankLines(lines []string) []string {
	var result []string
	prevWasBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
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

// trimTrailingEmptyLines removes trailing empty lines from a slice.
func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
