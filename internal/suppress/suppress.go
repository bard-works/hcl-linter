// Package suppress parses inline suppression comments in linted files:
//
//	# hcl-linter:ignore block_order,duplicates   (this line and the next)
//	# hcl-linter:ignore-file array_format         (whole file)
//
// `//` and `/* */` comments work too. With no rule list every rule is
// suppressed. Text after " -- " is a free-form reason and is ignored.
// Comments are found with the HCL lexer, so a directive-like string value is
// never mistaken for a directive.
package suppress

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/diag"
)

const (
	prefix     = "hcl-linter:"
	lineDir    = "ignore"
	fileDir    = "ignore-file"
	allRulesID = "*"
)

// Directives holds the suppressions declared in one file.
type Directives struct {
	file  map[string]bool
	lines map[int]map[string]bool
}

// Parse extracts suppression directives from file content.
//
// A line directive written after code on the same line covers that line. A
// line directive on a line of its own covers the next line containing code,
// so several directives can be stacked above one line.
func Parse(content []byte, filename string) Directives {
	d := Directives{}
	tokens, _ := hclsyntax.LexConfig(content, filename, hcl.InitialPos)
	lastCodeLine := 0
	var pending []string
	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenNewline, hclsyntax.TokenEOF:
			continue
		case hclsyntax.TokenComment:
		default:
			if len(pending) > 0 {
				d.addLine(tok.Range.Start.Line, pending)
				pending = nil
			}
			lastCodeLine = tok.Range.End.Line
			continue
		}
		kind, ruleIDs, ok := parseComment(string(tok.Bytes))
		if !ok {
			continue
		}
		switch {
		case kind == fileDir:
			if d.file == nil {
				d.file = map[string]bool{}
			}
			addAll(d.file, ruleIDs)
		case tok.Range.Start.Line == lastCodeLine:
			d.addLine(lastCodeLine, ruleIDs)
		default:
			pending = append(pending, ruleIDs...)
		}
	}
	return d
}

func (d *Directives) addLine(line int, ruleIDs []string) {
	if d.lines == nil {
		d.lines = map[int]map[string]bool{}
	}
	if d.lines[line] == nil {
		d.lines[line] = map[string]bool{}
	}
	addAll(d.lines[line], ruleIDs)
}

func parseComment(text string) (kind string, ruleIDs []string, ok bool) {
	text = strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(text, "#"):
		text = text[1:]
	case strings.HasPrefix(text, "//"):
		text = text[2:]
	case strings.HasPrefix(text, "/*"):
		text = strings.TrimSuffix(text[2:], "*/")
	}
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, prefix) {
		return "", nil, false
	}
	text = text[len(prefix):]
	if reason := strings.Index(text, " -- "); reason >= 0 {
		text = text[:reason]
	}
	fields := strings.Fields(strings.ReplaceAll(text, ",", " "))
	if len(fields) == 0 || (fields[0] != lineDir && fields[0] != fileDir) {
		return "", nil, false
	}
	ruleIDs = fields[1:]
	if len(ruleIDs) == 0 {
		ruleIDs = []string{allRulesID}
	}
	return fields[0], ruleIDs, true
}

func addAll(set map[string]bool, ids []string) {
	for _, id := range ids {
		set[id] = true
	}
}

func matches(set map[string]bool, names ...string) bool {
	if set[allRulesID] {
		return true
	}
	for _, n := range names {
		if set[n] {
			return true
		}
	}
	return false
}

// FileSuppressed reports whether ruleName is disabled for the whole file.
func (d Directives) FileSuppressed(ruleName string) bool {
	return matches(d.file, ruleName)
}

// Suppressed reports whether issue, produced by the rule ruleName, is covered
// by a directive. Directives match the rule's config name (block_order) or
// the emitted rule ID (dependency_path_exists). Internal linter errors are
// never suppressed.
func (d Directives) Suppressed(ruleName string, issue diag.Issue) bool {
	if issue.Rule == diag.RuleLinterError {
		return false
	}
	if matches(d.file, ruleName, issue.Rule) {
		return true
	}
	line := issue.Location.Start.Line
	return line > 0 && matches(d.lines[line], ruleName, issue.Rule)
}

// Filter drops the issues covered by a directive.
func (d Directives) Filter(ruleName string, issues []diag.Issue) []diag.Issue {
	if d.file == nil && d.lines == nil {
		return issues
	}
	kept := issues[:0]
	for _, is := range issues {
		if !d.Suppressed(ruleName, is) {
			kept = append(kept, is)
		}
	}
	return kept
}
