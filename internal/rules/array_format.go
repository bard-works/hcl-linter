package rules

import (
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type ArrayFormatRule struct{}

func (r ArrayFormatRule) Name() string  { return "array_format" }
func (r ArrayFormatRule) Priority() int { return PriorityFormat }

func init() { Register(ArrayFormatRule{}) }

func (r ArrayFormatRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Normalizes inline arrays with 2+ items to multiline format.",
		Severity:    "warning",
		Fixable:     true,
		ConfigBlock: "array_format",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{
				Name:     "sort",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Sort array items alphabetically when reformatting",
			},
		},
		Example: Example{
			Violation: `deps = ["a", "b", "c"]`,
			Fixed: `deps = [
  "a",
  "b",
  "c",
]`,
		},
	}
}

func (r ArrayFormatRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled
}

func (r ArrayFormatRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	checkArrayFormatBlocks(&issues, ctx.Blocks)
	return issues
}

func (r ArrayFormatRule) Fix(ctx *Context) (int, error) {
	sortItems := ctx.Config.ArrayFormat != nil && ctx.Config.ArrayFormat.Sort
	newContent, changes := FixArrays(string(ctx.Content), sortItems)
	if changes == 0 {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return changes, nil
}

var (
	// key = ["a", "b"]   (optionally followed by a comma inside an object)
	inlineArrayRe = regexp.MustCompile(`^(\s*)([A-Za-z_][\w-]*|"[^"]*")\s*=\s*\[(.*)\](\s*,?)\s*$`)
	// key = [
	arrayOpenRe = regexp.MustCompile(`^(\s*)([A-Za-z_][\w-]*|"[^"]*")\s*=\s*\[\s*$`)
	// ]   or   ],
	arrayCloseRe = regexp.MustCompile(`^\s*\](\s*,?)\s*$`)
)

// FixArrays converts single-line arrays with 2+ items to multiline format and
// adds missing trailing commas to multiline ones. When sortItems is true,
// items are sorted alphabetically.
//
// It works on text, so it only touches the shapes it can rewrite without
// changing meaning: a whole line that is `key = [...]` (or an array opening
// and closing on lines of their own) whose items are all plain string
// literals. Arrays of objects, function calls, interpolations, trailing
// comments, and anything inside heredocs are left alone.
func FixArrays(content string, sortItems bool) (string, int) {
	skip, ok := nonCodeLines(content)
	if !ok {
		return content, 0
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	changes := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if skip[i+1] {
			out = append(out, line)
			continue
		}
		if m := inlineArrayRe.FindStringSubmatch(line); m != nil {
			if items, ok := splitStringItems(m[3]); ok && len(items) >= 2 {
				out = append(out, renderArray(m[1], m[2], items, strings.TrimSpace(m[4]), sortItems)...)
				changes++
				continue
			}
		}
		if m := arrayOpenRe.FindStringSubmatch(line); m != nil {
			if end, items, suffix, ok := collectMultilineArray(lines, i, skip); ok && len(items) >= 2 {
				rendered := renderArray(m[1], m[2], items, suffix, sortItems)
				if !slices.Equal(rendered, lines[i:end+1]) {
					out = append(out, rendered...)
					changes++
					i = end
					continue
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), changes
}

// nonCodeLines returns the 1-based lines that belong to heredoc bodies or
// multi-line comments, which the text-based fixer must never edit. ok is false
// when the content does not lex cleanly.
func nonCodeLines(content string) (map[int]bool, bool) {
	tokens, diags := hclsyntax.LexConfig([]byte(content), "", hcl.InitialPos)
	if diags.HasErrors() {
		return nil, false
	}
	skip := map[int]bool{}
	heredocStart := 0
	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenOHeredoc:
			heredocStart = tok.Range.Start.Line
		case hclsyntax.TokenCHeredoc:
			for l := heredocStart; l <= tok.Range.End.Line; l++ {
				skip[l] = true
			}
		case hclsyntax.TokenComment:
			if tok.Range.End.Line > tok.Range.Start.Line+1 || strings.HasPrefix(string(tok.Bytes), "/*") {
				for l := tok.Range.Start.Line; l <= tok.Range.End.Line; l++ {
					skip[l] = true
				}
			}
		}
	}
	return skip, true
}

// collectMultilineArray reads the array opened on lines[start] up to its
// closing bracket. ok is false unless every line in between holds only plain
// string items.
func collectMultilineArray(lines []string, start int, skip map[int]bool) (end int, items []string, suffix string, ok bool) {
	for i := start + 1; i < len(lines); i++ {
		if skip[i+1] {
			return 0, nil, "", false
		}
		if m := arrayCloseRe.FindStringSubmatch(lines[i]); m != nil {
			return i, items, strings.TrimSpace(m[1]), true
		}
		body := strings.TrimSpace(lines[i])
		if body == "" {
			continue
		}
		lineItems, ok := splitStringItems(body)
		if !ok {
			return 0, nil, "", false
		}
		items = append(items, lineItems...)
	}
	return 0, nil, "", false
}

// splitStringItems splits a comma-separated list whose items are all plain
// quoted strings (no interpolation). ok is false for anything else.
func splitStringItems(inner string) ([]string, bool) {
	var items []string
	var cur strings.Builder
	inQuote, escaped := false, false
	for _, ch := range inner {
		switch {
		case inQuote:
			cur.WriteRune(ch)
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inQuote = false
			}
		case ch == '"':
			if strings.TrimSpace(cur.String()) != "" {
				return nil, false // text before the opening quote
			}
			cur.Reset()
			cur.WriteRune(ch)
			inQuote = true
		case ch == ',':
			if item := strings.TrimSpace(cur.String()); item != "" {
				items = append(items, item)
			}
			cur.Reset()
		case ch == ' ' || ch == '\t':
			cur.WriteRune(ch)
		default:
			return nil, false // unquoted token: reference, call, number, comment...
		}
	}
	if inQuote {
		return nil, false
	}
	if item := strings.TrimSpace(cur.String()); item != "" {
		items = append(items, item)
	}
	for _, item := range items {
		if strings.Contains(item, "${") || strings.Contains(item, "%{") {
			return nil, false
		}
	}
	return items, true
}

func renderArray(indent, key string, items []string, suffix string, sortItems bool) []string {
	if sortItems {
		items = slices.Clone(items)
		sort.Strings(items)
	}
	out := []string{indent + key + " = ["}
	for _, item := range items {
		out = append(out, indent+"  "+item+",")
	}
	return append(out, indent+"]"+suffix)
}

// --- Check helpers ---

func checkArrayFormatBlocks(issues *[]diag.Issue, blocks []ast.BlockInfo) {
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

func checkExprArrayFormat(issues *[]diag.Issue, expr hclsyntax.Expression) {
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
				*issues = append(*issues, diag.Issue{
					Severity: diag.SeverityWarning,
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
