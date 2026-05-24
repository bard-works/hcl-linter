package rules

import (
	"fmt"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type RequiredFieldsRule struct{}

func (r RequiredFieldsRule) Name() string  { return "required_fields" }
func (r RequiredFieldsRule) Priority() int { return PrioritySemantic }

func init() { Register(RequiredFieldsRule{}) }

func (r RequiredFieldsRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Enforces required attributes on specific block types.",
		Severity:    "error",
		Fixable:     true,
		ConfigBlock: "required_fields",
		ConfigFields: []ConfigField{
			{
				Name:     "include.expose",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Require expose = true on every include block",
			},
		},
		Example: Example{
			Violation: `include "root" { path = find_in_parent_folders() }`,
			Fixed: `include "root" {
  path   = find_in_parent_folders()
  expose = true
}`,
		},
	}
}

func (r RequiredFieldsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.RequiredFields != nil
}

func (r RequiredFieldsRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	checkRequiredFields(&issues, ctx.Blocks, ctx.Config.RequiredFields)
	return issues
}

func (r RequiredFieldsRule) Fix(ctx *Context) (int, error) {
	newContent, changed := FixRequiredFields(string(ctx.Content), ctx.Blocks, ctx.Config.RequiredFields)
	if !changed {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return 1, nil
}

// FixRequiredFields adds missing required attributes to blocks.
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

func checkRequiredFields(issues *[]diag.Issue, blocks []ast.BlockInfo, cfg *config.RequiredFieldsConfig) {
	for _, block := range blocks {
		if block.Type == "include" {
			if cfg.Include != nil && cfg.Include.Expose {
				attrs := ast.GetBlockAttributes(block.Block.Body)
				if _, ok := attrs["expose"]; !ok {
					*issues = append(*issues, diag.Issue{
						Severity:     diag.SeverityError,
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

func blockIndent(lines []string, lineIdx int) string {
	if lineIdx >= len(lines) {
		return ""
	}
	var sb strings.Builder
	for _, ch := range lines[lineIdx] {
		if ch != ' ' && ch != '\t' {
			break
		}
		sb.WriteRune(ch)
	}
	return sb.String()
}

func expandSingleLineBlock(lines []string, startLine int, attr string) (string, bool) {
	line := lines[startLine]
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "{") || !strings.Contains(trimmed, "}") {
		return "", false
	}
	braceIdx := -1
	for i, ch := range line {
		if ch == '{' {
			braceIdx = i
			break
		}
	}
	if braceIdx < 0 {
		return "", false
	}
	indent := blockIndent(lines, startLine)
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
	return strings.Join(lines, "\n"), true
}

func findInsertIdx(lines []string, startLine, endLine int) int {
	for i := startLine + 1; i <= endLine && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "}" || strings.HasPrefix(trimmed, "}") {
			return i
		}
	}
	return startLine + 1
}

func contentIndentForBlock(lines []string, startLine, insertIdx int) string {
	indent := blockIndent(lines, startLine)
	contentIndent := indent + "  "
	for i := startLine + 1; i < insertIdx && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			existingIndent := blockIndent(lines, i)
			if len(existingIndent) >= 2 {
				return indent + existingIndent[:2]
			}
			break
		}
	}
	return contentIndent
}

// addAttributeToBlock inserts attr as a new line inside block's braces.
func addAttributeToBlock(content string, block ast.BlockInfo, attr string) string {
	startLine := block.StartLine
	endLine := block.EndLine

	lines := strings.Split(content, "\n")

	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	if startLine == endLine {
		if result, ok := expandSingleLineBlock(lines, startLine, attr); ok {
			return result
		}
	}

	insertIdx := findInsertIdx(lines, startLine, endLine)
	contentIndent := contentIndentForBlock(lines, startLine, insertIdx)

	var newLines []string
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, contentIndent+attr)
	if insertIdx < len(lines) {
		newLines = append(newLines, lines[insertIdx:]...)
	}
	return strings.Join(newLines, "\n")
}
