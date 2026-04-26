package rules

import (
	"fmt"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
)

type RequiredFieldsRule struct{}

func (r RequiredFieldsRule) Name() string { return "required_fields" }

func (r RequiredFieldsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.RequiredFields != nil
}

func (r RequiredFieldsRule) Check(ctx *Context) []linter.Issue {
	var issues []linter.Issue
	checkRequiredFields(&issues, ctx.Blocks, ctx.Config.RequiredFields)
	return issues
}

func (r RequiredFieldsRule) Fix(ctx *Context) ([]byte, bool, error) {
	newContent, changed := FixRequiredFields(string(ctx.Content), ctx.Blocks, ctx.Config.RequiredFields)
	if !changed {
		return ctx.Content, false, nil
	}
	return []byte(newContent), true, nil
}

// FixRequiredFields adds missing required attributes to blocks.
// Exported for use by the legacy fixer during migration.
func FixRequiredFields(content string, blocks []ast.BlockInfo, cfg *config.RequiredFieldsConfig) (string, bool) {
	if cfg == nil || cfg.Include == nil || !cfg.Include.Expose {
		return content, false
	}
	changed := false
	for _, block := range blocks {
		if block.Type == "include" && len(block.Labels) > 0 {
			attrs := ast.GetBlockAttributes(block.Block.Body)
			if _, ok := attrs["expose"]; !ok {
				content = addAttributeToBlock(content, block, "expose = true")
				changed = true
			}
		}
	}
	return content, changed
}

func checkRequiredFields(issues *[]linter.Issue, blocks []ast.BlockInfo, cfg *config.RequiredFieldsConfig) {
	for _, block := range blocks {
		if block.Type == "include" {
			if cfg.Include != nil && cfg.Include.Expose {
				attrs := ast.GetBlockAttributes(block.Block.Body)
				if _, ok := attrs["expose"]; !ok {
					*issues = append(*issues, linter.Issue{
						Severity:     linter.SeverityError,
						Rule:         "required_fields",
						Message:      fmt.Sprintf("include block %q missing required field 'expose'", block.Labels),
						Location:     block.Block.TypeRange,
						SuggestedFix: "expose = true",
					})
				}
			}
		}
		if len(block.Block.Body.Blocks) > 0 {
			checkRequiredFields(issues, ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks), cfg)
		}
	}
}

// addAttributeToBlock inserts attr as a new line inside block's braces.
func addAttributeToBlock(content string, block ast.BlockInfo, attr string) string {
	startLine := block.StartLine
	endLine := block.EndLine

	lines := strings.Split(content, "\n")

	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	indent := ""
	if startLine < len(lines) {
		var sb strings.Builder
		for _, ch := range lines[startLine] {
			if ch != ' ' && ch != '\t' {
				break
			}
			sb.WriteRune(ch)
		}
		indent = sb.String()
	}

	if startLine == endLine {
		line := lines[startLine]
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
			braceIdx := -1
			for i, ch := range line {
				if ch == '{' {
					braceIdx = i
					break
				}
			}
			if braceIdx >= 0 {
				beforeBrace := strings.TrimRight(line[:braceIdx], " \t")
				contentIndent := "  "
				if indent != "" {
					contentIndent = indent + "  "
				}
				lines[startLine] = strings.Join([]string{
					beforeBrace + " {",
					contentIndent + attr,
					"}",
				}, "\n")
				return strings.Join(lines, "\n")
			}
		}
	}

	insertIdx := startLine + 1
	for i := startLine + 1; i <= endLine && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "}" || strings.HasPrefix(trimmed, "}") {
			insertIdx = i
			break
		}
	}

	contentIndent := indent + "  "
	for i := startLine + 1; i < insertIdx && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			var sb strings.Builder
			for _, ch := range lines[i] {
				if ch != ' ' && ch != '\t' {
					break
				}
				sb.WriteRune(ch)
			}
			existingIndent := sb.String()
			if len(existingIndent) >= 2 {
				contentIndent = indent + existingIndent[:2]
			}
			break
		}
	}

	var newLines []string
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, contentIndent+attr)
	if insertIdx < len(lines) {
		newLines = append(newLines, lines[insertIdx:]...)
	}
	return strings.Join(newLines, "\n")
}
