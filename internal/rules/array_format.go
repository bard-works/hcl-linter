package rules

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

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

// Check reports exactly the arrays Fix rewrites, found by running the fixer:
// a file `check` passes is one `fix` leaves alone, and the reverse.
func (r ArrayFormatRule) Check(ctx *Context) []diag.Issue {
	_, edits := r.fix(ctx)
	issues := make([]diag.Issue, 0, len(edits))
	for _, e := range edits {
		issues = append(issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "array_format",
			Message:  e.message,
			Location: hcl.Range{
				Filename: ctx.FilePath,
				Start:    hcl.Pos{Line: e.line, Column: e.column},
				End:      hcl.Pos{Line: e.endLine, Column: e.endColumn},
			},
		})
	}
	return issues
}

func (r ArrayFormatRule) Fix(ctx *Context) (int, error) {
	newContent, edits := r.fix(ctx)
	if len(edits) == 0 {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return len(edits), nil
}

func (r ArrayFormatRule) fix(ctx *Context) (string, []arrayEdit) {
	cfg := ctx.Config.ArrayFormat
	suppressed := func(line int) bool { return ctx.Directives.LineSuppressed(line, r.Name()) }
	return fixArrays(string(ctx.Content), cfg != nil && cfg.Sort, arrayThreshold(cfg), suppressed)
}

// arrayThreshold is the item count from which arrays go one item per line.
func arrayThreshold(cfg *config.ArrayFormatConfig) int {
	if cfg == nil || cfg.MultilineThreshold < 1 {
		return 2
	}
	return cfg.MultilineThreshold
}

var (
	// Matches `key = ["a", "b"]`, optionally followed by a comma inside an object.
	inlineArrayRe = regexp.MustCompile(`^(\s*)([A-Za-z_][\w-]*|"[^"]*")\s*=\s*\[(.*)\](\s*,?)\s*$`)
	// Matches `key = [` opening a multiline array.
	arrayOpenRe = regexp.MustCompile(`^(\s*)([A-Za-z_][\w-]*|"[^"]*")\s*=\s*\[\s*$`)
	// Matches the closing `]` or `],`.
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
	out, edits := fixArrays(content, sortItems, 2, nil)
	return out, len(edits)
}

// arrayEdit is one array fixArrays rewrites: where it was and why.
type arrayEdit struct {
	line, column, endLine, endColumn int
	message                          string
}

// fixArrays rewrites arrays and returns one edit per array changed. Lines
// for which suppressed (may be nil) returns true are left alone.
func fixArrays(content string, sortItems bool, threshold int, suppressed func(line int) bool) (string, []arrayEdit) {
	skip, ok := nonCodeLines(content)
	if !ok {
		return content, nil
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	var edits []arrayEdit

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if skip[i+1] || (suppressed != nil && suppressed(i+1)) {
			out = append(out, line)
			continue
		}
		if m := inlineArrayRe.FindStringSubmatchIndex(line); m != nil {
			indent, key, inner, suffix := line[m[2]:m[3]], line[m[4]:m[5]], line[m[6]:m[7]], line[m[8]:m[9]]
			if items, ok := splitStringItems(inner); ok && len(items) >= threshold {
				out = append(out, renderArray(indent, key, items, strings.TrimSpace(suffix), sortItems)...)
				edits = append(edits, arrayEdit{
					line: i + 1, column: m[6], // the "[" just before the items
					endLine: i + 1, endColumn: m[7] + 2, // just past the "]"
					message: fmt.Sprintf("array with %d items should list one item per line", len(items)),
				})
				continue
			}
		}
		if m := arrayOpenRe.FindStringSubmatch(line); m != nil {
			if end, items, suffix, ok := collectMultilineArray(lines, i, skip); ok && len(items) >= 2 {
				rendered := renderArray(m[1], m[2], items, suffix, sortItems)
				if !slices.Equal(rendered, lines[i:end+1]) {
					message := "multiline array should list one item per line, each followed by a comma"
					if sortItems && !slices.IsSorted(items) {
						message = "array items should be sorted"
					}
					out = append(out, rendered...)
					edits = append(edits, arrayEdit{
						line: i + 1, column: strings.LastIndex(line, "[") + 1,
						endLine: end + 1, endColumn: strings.LastIndex(lines[end], "]") + 2,
						message: message,
					})
					i = end
					continue
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), edits
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
func collectMultilineArray(
	lines []string,
	start int,
	skip map[int]bool,
) (end int, items []string, suffix string, ok bool) {
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
