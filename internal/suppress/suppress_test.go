package suppress

import (
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/bard-works/hcl-linter/internal/diag"
)

func issueAt(rule string, line int) diag.Issue {
	return diag.Issue{Rule: rule, Severity: diag.SeverityError, Location: hcl.Range{Start: hcl.Pos{Line: line}}}
}

func TestSuppressionDirectives(t *testing.T) {
	src := `# hcl-linter:ignore-file array_format -- legacy file
# hcl-linter:ignore block_order
terraform {}
include "root" {} // hcl-linter:ignore
dependency "a" {}
/* hcl-linter:ignore dependency_path_exists, duplicates */
dependency "b" {}
locals {
  note = "# hcl-linter:ignore-file"
}
`
	d := Parse([]byte(src), "t.hcl")
	tests := []struct {
		name  string
		rule  string
		issue diag.Issue
		want  bool
	}{
		{"file-wide by rule name", "array_format", issueAt("array_format", 9), true},
		{"file-wide does not cover other rules", "duplicates", issueAt("duplicates", 5), false},
		{"comment above covers next line", "block_order", issueAt("block_order", 3), true},
		{"comment above names another rule", "duplicates", issueAt("duplicates", 3), false},
		{"trailing comment with no list covers all rules", "duplicates", issueAt("duplicates", 4), true},
		{"trailing comment does not reach the next line", "block_order", issueAt("block_order", 5), false},
		{"emitted rule ID matches", "dependency_paths", issueAt("dependency_path_exists", 7), true},
		{"unrelated line", "block_order", issueAt("block_order", 8), false},
		{"linter errors are never suppressed", "array_format", issueAt(diag.RuleLinterError, 3), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.Suppressed(tt.rule, tt.issue); got != tt.want {
				t.Errorf("Suppressed = %v, want %v", got, tt.want)
			}
		})
	}
	if d.FileSuppressed("block_order") {
		t.Error("a directive inside a string value must not be treated as a comment")
	}
}

func TestParseIgnoresOrdinaryComments(t *testing.T) {
	d := Parse([]byte("# just a note about hcl-linter\nlocals {}\n# hcl-linter:ignoreX foo\n"), "t.hcl")
	if d.file != nil || d.lines != nil {
		t.Errorf("unexpected directives: %+v", d)
	}
}

func TestStackedDirectivesCoverNextCodeLine(t *testing.T) {
	src := "# hcl-linter:ignore block_order\n# hcl-linter:ignore duplicates\n\ninclude \"a\" {}\n"
	d := Parse([]byte(src), "t.hcl")
	for _, rule := range []string{"block_order", "duplicates"} {
		if !d.Suppressed(rule, issueAt(rule, 4)) {
			t.Errorf("%s on line 4 should be suppressed", rule)
		}
	}
}

func TestDirectiveAboveBlockCoversWholeBlock(t *testing.T) {
	src := `# hcl-linter:ignore dependency_path_exists
dependency "bootstrap" {
  config_path = "../bootstrap"
  mock_outputs = {
    id = "x"
  }
}
dependency "other" {
  config_path = "../other"
}
`
	d := Parse([]byte(src), "t.hcl")
	for line := 2; line <= 7; line++ {
		if !d.Suppressed("dependency_paths", issueAt("dependency_path_exists", line)) {
			t.Errorf("line %d inside the annotated block should be suppressed", line)
		}
	}
	if d.Suppressed("dependency_paths", issueAt("dependency_path_exists", 9)) {
		t.Error("the next block must not be covered")
	}
}

func TestLineSuppressed(t *testing.T) {
	src := `# hcl-linter:ignore array_format
inputs = {
  azs = ["a", "b"]
}
other = ["c", "d"]
`
	d := Parse([]byte(src), "t.hcl")
	if !d.LineSuppressed(3, "array_format") {
		t.Error("line 3 is inside the annotated attribute")
	}
	if d.LineSuppressed(3, "blank_lines") {
		t.Error("the directive names array_format only")
	}
	if d.LineSuppressed(5, "array_format") {
		t.Error("line 5 follows the annotated attribute")
	}
	if !Parse([]byte("# hcl-linter:ignore-file\na = 1\n"), "t.hcl").LineSuppressed(2, "array_format") {
		t.Error("ignore-file covers every line")
	}
}
