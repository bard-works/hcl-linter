package fix

import (
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
)

// fixBlankLinesWithinBlocks removes unnecessary blank lines within blocks and object attributes.
func fixBlankLinesWithinBlocks(content string, blocks []ast.BlockInfo, attrs []ast.AttributeInfo) (string, int) {
	lines := strings.Split(content, "\n")
	changes := 0
	usedLines := make(map[int]bool)
	for _, block := range blocks {
		for l := block.StartLine; l <= block.EndLine; l++ {
			usedLines[l] = true
		}
	}
	type objectAttr struct {
		attr      ast.AttributeInfo
		firstLine int
	}
	var objectAttrs []objectAttr
	for _, attr := range attrs {
		if ast.IsObjectAttribute(attr.Expr) {
			startLine, endLine := ast.GetAttributeRange(attr.Expr)
			for l := startLine; l <= endLine; l++ {
				usedLines[l] = true
			}
			objectAttrs = append(objectAttrs, objectAttr{attr, startLine})
		}
	}
	var result []string
	processedLines := make(map[int]bool)
	i := 0
	for i < len(lines) {
		lineIdx := i
		if usedLines[lineIdx] && !processedLines[lineIdx] {
			var contentStart, contentEnd int
			var prefixLines []string
			isObjAttr := false
			for _, oa := range objectAttrs {
				if oa.firstLine != lineIdx {
					continue
				}
				_, contentEnd = ast.GetAttributeRange(oa.attr.Expr)
				prefixLines = []string{lines[lineIdx]}
				contentStart = lineIdx + 1
				isObjAttr = true
			}
			if isObjAttr {
				for j := lineIdx; j <= contentEnd; j++ {
					processedLines[j] = true
				}
				var blockLines []string
				blockLines = append(blockLines, prefixLines...)
				for j := contentStart; j <= contentEnd; j++ {
					blockLines = append(blockLines, lines[j])
				}
				fixedBlockLines := removeBlankLinesWithinBlock(blockLines)
				if len(fixedBlockLines) != len(blockLines) {
					changes++
				}
				result = append(result, fixedBlockLines...)
				i = contentEnd + 1
				continue
			}
			hasNestedBlock := false
			for _, block := range blocks {
				if block.StartLine != lineIdx {
					continue
				}
				prefixLines = []string{lines[lineIdx]}
				contentStart = blockContentStart(lines, lineIdx)
				contentEnd = block.EndLine
				for j := lineIdx; j <= contentEnd; j++ {
					processedLines[j] = true
				}
				if contentStart < contentEnd {
					for k := contentStart; k < contentEnd; k++ {
						for _, b := range blocks {
							if b.StartLine == k {
								hasNestedBlock = true
								break
							}
						}
						if hasNestedBlock {
							break
						}
					}
				}
			}
			if len(prefixLines) > 0 && contentEnd > contentStart && !hasNestedBlock {
				var blockLines []string
				blockLines = append(blockLines, prefixLines...)
				for j := contentStart; j <= contentEnd; j++ {
					blockLines = append(blockLines, lines[j])
				}
				fixedBlockLines := removeBlankLinesWithinBlock(blockLines)
				if len(fixedBlockLines) != len(blockLines) {
					changes++
				}
				result = append(result, fixedBlockLines...)
				i = contentEnd + 1
				continue
			}
		}
		result = append(result, lines[i])
		processedLines[i] = true
		i++
	}
	return strings.Join(result, "\n") + "\n", changes
}

// blockContentStart finds the line where a block's content starts (after the opening brace).
func blockContentStart(lines []string, blockStart int) int {
	for i := blockStart; i < len(lines); i++ {
		if strings.Contains(strings.TrimSpace(lines[i]), "{") {
			return i + 1
		}
	}
	return blockStart + 1
}

// removeBlankLinesWithinBlock removes unnecessary blank lines within a block.
func removeBlankLinesWithinBlock(lines []string) []string {
	if len(lines) < 2 {
		return lines
	}
	var result []string
	var lastWasBlank bool
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		isLastLine := i == len(lines)-1
		isFirstLine := i == 0
		isBlockHeader := isFirstLine && strings.Contains(line, "{")

		if trimmed == "" {
			if isLastLine {
				continue
			}
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if nextTrimmed == "}" || strings.HasPrefix(nextTrimmed, "}") {
				continue
			}
			if isBlockHeader {
				continue
			}
			if !lastWasBlank {
				result = append(result, line)
				lastWasBlank = true
			}
		} else {
			result = append(result, line)
			lastWasBlank = false
		}
	}
	return result
}
