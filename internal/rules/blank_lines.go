package rules

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"

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
		Summary:     "Removes excess blank lines within blocks and object attributes.",
		Severity:    "warning",
		Fixable:     true,
		ConfigBlock: "blank_lines",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{
				Name:     "within_blocks",
				Type:     "bool",
				Required: true,
				Doc:      "Remove blank lines inside block bodies and object attributes",
			},
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

// Check reports what Fix would change, so `check` in CI catches the same
// drift as `fix --dry-run`: one warning at the first line Fix would remove.
func (r BlankLinesRule) Check(ctx *Context) []diag.Issue {
	fixed, changes := FixBlankLines(string(ctx.Content), ctx.Blocks, ctx.Attrs)
	if changes == 0 || fixed == string(ctx.Content) {
		return nil
	}
	before := strings.Split(string(ctx.Content), "\n")
	after := strings.Split(fixed, "\n")
	line := 1
	for line <= len(before) && line <= len(after) && before[line-1] == after[line-1] {
		line++
	}
	removed := len(before) - len(after)
	pos := hcl.Pos{Line: line, Column: 1}
	return []diag.Issue{{
		Severity: diag.SeverityWarning,
		Rule:     "blank_lines",
		Message:  fmt.Sprintf("%d extra blank line(s) inside blocks; run `hcl-linter fix` to remove", removed),
		Location: hcl.Range{Filename: ctx.FilePath, Start: pos, End: pos},
	}}
}

func (r BlankLinesRule) Fix(ctx *Context) (int, error) {
	newContent, changes := FixBlankLines(string(ctx.Content), ctx.Blocks, ctx.Attrs)
	if changes == 0 {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return changes, nil
}

type objectAttr struct {
	attr      ast.AttributeInfo
	firstLine int
}

func buildUsedLines(blocks []ast.BlockInfo, attrs []ast.AttributeInfo) (map[int]bool, []objectAttr) {
	used := make(map[int]bool)
	for _, block := range blocks {
		for l := block.StartLine; l <= block.EndLine; l++ {
			used[l] = true
		}
	}
	var objAttrs []objectAttr
	for _, attr := range attrs {
		if ast.IsObjectAttribute(attr.Expr) {
			startLine, endLine := ast.GetAttributeRange(attr.Expr)
			for l := startLine; l <= endLine; l++ {
				used[l] = true
			}
			objAttrs = append(objAttrs, objectAttr{attr, startLine})
		}
	}
	return used, objAttrs
}

func processObjectAttrBlock(lines []string, lineIdx int, contentEnd int) ([]string, int) {
	blockLines := append([]string{lines[lineIdx]}, lines[lineIdx+1:contentEnd+1]...)
	fixed := removeBlankLinesWithinBlock(blockLines)
	changes := 0
	if len(fixed) != len(blockLines) {
		changes++
	}
	return fixed, changes
}

func hasNestedBlockAt(blocks []ast.BlockInfo, start, end int) bool {
	for k := start; k < end; k++ {
		for _, b := range blocks {
			if b.StartLine == k {
				return true
			}
		}
	}
	return false
}

func processBlockContent(lines []string, lineIdx int, block ast.BlockInfo) ([]string, int, int) {
	contentStart := blankLinesBlockContentStart(lines, lineIdx)
	contentEnd := block.EndLine
	blockLines := append([]string{lines[lineIdx]}, lines[contentStart:contentEnd+1]...)
	fixed := removeBlankLinesWithinBlock(blockLines)
	changes := 0
	// Compare against the whole original span: blank lines skipped between
	// the opening line and contentStart are removals too.
	if len(fixed) != contentEnd-lineIdx+1 {
		changes++
	}
	return fixed, changes, contentEnd
}

// FixBlankLines removes unnecessary blank lines within blocks and object attributes.
func FixBlankLines(content string, blocks []ast.BlockInfo, attrs []ast.AttributeInfo) (string, int) {
	lines := strings.Split(content, "\n")
	usedLines, objectAttrs := buildUsedLines(blocks, attrs)

	var result []string
	processedLines := make(map[int]bool)
	changes := 0
	i := 0
	for i < len(lines) {
		if usedLines[i] && !processedLines[i] {
			for _, oa := range objectAttrs {
				if oa.firstLine != i {
					continue
				}
				_, endLine := ast.GetAttributeRange(oa.attr.Expr)
				for j := i; j <= endLine; j++ {
					processedLines[j] = true
				}
				fixed, c := processObjectAttrBlock(lines, i, endLine)
				changes += c
				result = append(result, fixed...)
				i = endLine + 1
				goto nextLine
			}
			for _, block := range blocks {
				if block.StartLine != i {
					continue
				}
				for j := i; j <= block.EndLine; j++ {
					processedLines[j] = true
				}
				cs := blankLinesBlockContentStart(lines, i)
				if cs < block.EndLine && !hasNestedBlockAt(blocks, cs, block.EndLine) {
					fixed, c, _ := processBlockContent(lines, i, block)
					changes += c
					result = append(result, fixed...)
					i = block.EndLine + 1
					goto nextLine
				}
			}
		}
		result = append(result, lines[i])
		processedLines[i] = true
		i++
	nextLine:
	}
	// Split/Join round-trips the file ending exactly; appending another
	// newline here grew the file by one blank line on every run.
	return strings.Join(result, "\n"), changes
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
			// Blank line right after an opening brace (block or object).
			if len(result) > 0 && strings.HasSuffix(strings.TrimSpace(result[len(result)-1]), "{") {
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
