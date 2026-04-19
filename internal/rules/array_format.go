package rules

import (
	"sort"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type ArrayFormatRule struct{}

func (r ArrayFormatRule) Name() string { return "array_format" }

func (r ArrayFormatRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled
}

func (r ArrayFormatRule) Check(ctx *Context) []linter.Issue {
	var issues []linter.Issue
	checkArrayFormatBlocks(&issues, ctx.Blocks)
	return issues
}

func (r ArrayFormatRule) Fix(ctx *Context) ([]byte, bool, error) {
	sortItems := ctx.Config.ArrayFormat != nil && ctx.Config.ArrayFormat.Sort
	newContent, changes := FixArrays(string(ctx.Content), sortItems)
	if changes == 0 {
		return ctx.Content, false, nil
	}
	return []byte(newContent), true, nil
}

// FixArrays converts single-line arrays with 2+ items to multiline format.
// When sortItems is true, items are sorted alphabetically.
// Exported for use by the legacy fixer during migration.
func FixArrays(content string, sortItems bool) (string, int) {
	lines := strings.Split(content, "\n")
	changes := 0
	var result strings.Builder

	i := 0
	for i < len(lines) {
		line := lines[i]

		if isArrayAssignment(line) && !isMultilineArrayStart(lines, i) {
			items := extractAndFormatArray(line)
			if len(items) >= 2 {
				result.WriteString(formatMultilineArray(line, items, sortItems))
				changes++
				i++
				continue
			}
		}

		if isMultilineArrayStart(lines, i) {
			arrayLines, itemCount := collectMultilineArray(lines, i)
			if itemCount >= 2 && needsMultilineFix(arrayLines) {
				fixed := fixMultilineArray(arrayLines, line, sortItems)
				result.WriteString(fixed)
				changes++
				i += len(arrayLines)
				continue
			}
			for _, l := range arrayLines {
				result.WriteString(l)
				result.WriteString("\n")
			}
			i += len(arrayLines)
			continue
		}

		result.WriteString(line)
		result.WriteString("\n")
		i++
	}

	return result.String(), changes
}

func isArrayAssignment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "=") && strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]")
}

func isMultilineArrayStart(lines []string, idx int) bool {
	if idx >= len(lines) {
		return false
	}
	trimmed := strings.TrimSpace(lines[idx])
	return strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "]")
}

func collectMultilineArray(lines []string, start int) ([]string, int) {
	var arrayLines []string
	itemCount := 0
	braceCount := 0

	for i := start; i < len(lines); i++ {
		line := lines[i]
		arrayLines = append(arrayLines, line)

		braceCount += strings.Count(line, "[") - strings.Count(line, "]")
		if strings.Contains(line, "\"") {
			itemCount += strings.Count(line, "\"")
		}

		if braceCount <= 0 && strings.Contains(line, "]") {
			break
		}
	}

	return arrayLines, itemCount / 2
}

func extractAndFormatArray(line string) []string {
	trimmed := strings.TrimSpace(line)

	start := strings.Index(trimmed, "[")
	end := strings.LastIndex(trimmed, "]")
	if start == -1 || end == -1 || end <= start {
		return nil
	}

	inner := trimmed[start+1 : end]
	var items []string
	inQuote := false
	var current strings.Builder

	for _, ch := range inner {
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ch == ',' && !inQuote:
			item := strings.TrimSpace(current.String())
			if item != "" {
				items = append(items, item)
			}
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}

	if item := strings.TrimSpace(current.String()); item != "" {
		items = append(items, item)
	}

	return items
}

func formatMultilineArray(line string, items []string, sortItems bool) string {
	indent := ""
	var indentBuilder strings.Builder
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			break
		}
		indentBuilder.WriteRune(ch)
	}
	indent = indentBuilder.String()

	idx := strings.Index(line, "[")
	if idx == -1 {
		return ""
	}
	key := strings.TrimSpace(line[:idx])

	if sortItems {
		sort.Strings(items)
	}

	var sb strings.Builder
	sb.WriteString(indent)
	sb.WriteString(key)
	sb.WriteString(" = [\n")
	for _, item := range items {
		sb.WriteString(indent)
		sb.WriteString("  ")
		sb.WriteString(strings.TrimSpace(item))
		sb.WriteString(",\n")
	}
	sb.WriteString(indent)
	sb.WriteString("]\n")
	return sb.String()
}

func needsMultilineFix(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ",") {
			return false
		}
		if strings.Count(trimmed, "\"") > 0 && !strings.Contains(trimmed, "]") {
			return true
		}
	}
	return false
}

func fixMultilineArray(lines []string, firstLine string, sortItems bool) string {
	indent := ""
	var indentBuilder strings.Builder
	for _, ch := range firstLine {
		if ch != ' ' && ch != '\t' {
			break
		}
		indentBuilder.WriteRune(ch)
	}
	indent = indentBuilder.String()

	idx := strings.Index(firstLine, "[")
	if idx == -1 {
		return ""
	}
	key := strings.TrimSpace(firstLine[:idx])

	var items []string
	for _, line := range lines[1 : len(lines)-1] {
		trimmed := strings.TrimSuffix(strings.TrimSpace(line), ",")
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	if sortItems {
		sort.Strings(items)
	}

	var sb strings.Builder
	sb.WriteString(indent)
	sb.WriteString(key)
	sb.WriteString(" = [\n")
	for _, item := range items {
		sb.WriteString(indent)
		sb.WriteString("  ")
		sb.WriteString(item)
		sb.WriteString(",\n")
	}
	sb.WriteString(indent)
	sb.WriteString("]\n")
	return sb.String()
}

// --- Check helpers ---

func checkArrayFormatBlocks(issues *[]linter.Issue, blocks []ast.BlockInfo) {
	for _, block := range blocks {
		if len(block.Block.Body.Attributes) > 0 {
			for _, attr := range block.Block.Body.Attributes {
				checkExprArrayFormat(issues, attr.Expr)
			}
		}
		if len(block.Block.Body.Blocks) > 0 {
			checkArrayFormatBlocks(issues, ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks))
		}
	}
}

func checkExprArrayFormat(issues *[]linter.Issue, expr hclsyntax.Expression) {
	switch e := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		if len(e.Exprs) > 1 {
			allStrings := true
			for _, ex := range e.Exprs {
				if !isStringLiteralExpr(ex) {
					allStrings = false
					break
				}
			}
			if allStrings {
				*issues = append(*issues, linter.Issue{
					Severity: linter.SeverityWarning,
					Rule:     "array_format",
					Message:  "tuple of string literals should be written as a list for better readability",
				})
			}
		}
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			checkExprArrayFormat(issues, item.ValueExpr)
		}
	}
}

func isStringLiteralExpr(expr hclsyntax.Expression) bool {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return e.Val.Type() == cty.String
	case *hclsyntax.TemplateExpr:
		return true
	default:
		return false
	}
}
