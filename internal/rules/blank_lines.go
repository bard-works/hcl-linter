package rules

import (
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type BlankLinesRule struct{}

func (r BlankLinesRule) Name() string  { return "blank_lines" }
func (r BlankLinesRule) Priority() int { return PriorityFinal }

func init() { Register(BlankLinesRule{}) }

func (r BlankLinesRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Removes excess blank lines within blocks and object attributes (fix-only).",
		Severity:    "warning",
		Fixable:     true,
		ConfigBlock: "blank_lines",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{Name: "within_blocks", Type: "bool", Required: true, Doc: "Remove blank lines inside block bodies and object attributes"},
		},
		Example: Example{
			Violation: `inputs = {
  name = "foo"

}`,
			Fixed: `inputs = {
  name = "foo"
}`,
		},
	}
}

func (r BlankLinesRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.BlankLines != nil && cfg.BlankLines.Enabled && cfg.BlankLines.WithinBlocks
}

func (r BlankLinesRule) Check(_ *Context) []diag.Issue { return nil }

func (r BlankLinesRule) Fix(ctx *Context) (int, error) {
	newContent, changes := FixBlankLines(string(ctx.Content), ctx.Blocks, ctx.Attrs)
	if changes == 0 {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return changes, nil
}

// FixBlankLines removes unnecessary blank lines within blocks and object attributes.
func FixBlankLines(content string, blocks []ast.BlockInfo, attrs []ast.AttributeInfo) (string, int) {
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
				contentStart = blankLinesBlockContentStart(lines, lineIdx)
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

func blankLinesBlockContentStart(lines []string, blockStart int) int {
	for i := blockStart; i < len(lines); i++ {
		if strings.Contains(strings.TrimSpace(lines[i]), "{") {
			return i + 1
		}
	}
	return blockStart + 1
}

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
