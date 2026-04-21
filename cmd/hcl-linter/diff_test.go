package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/termcolor"
)

// captureDiff redirects diffOut to a buffer for the duration of the test.
func captureDiff(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := diffOut
	buf := &bytes.Buffer{}
	diffOut = buf
	t.Cleanup(func() { diffOut = prev })
	return buf
}

func TestPrintDiffIdentical(t *testing.T) {
	buf := captureDiff(t)

	changed := printDiff("foo.hcl", "a\nb\n", "a\nb\n")
	if changed {
		t.Error("expected printDiff to return false for identical input")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestPrintDiffDetectsChange(t *testing.T) {
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	buf := captureDiff(t)

	before := "line1\nold\nline3\n"
	after := "line1\nnew\nline3\n"

	changed := printDiff("foo.hcl", before, after)
	if !changed {
		t.Fatal("expected printDiff to return true for differing input")
	}

	out := buf.String()
	if !strings.Contains(out, "--- a/foo.hcl") {
		t.Errorf("expected '--- a/foo.hcl' header, got:\n%s", out)
	}
	if !strings.Contains(out, "+++ b/foo.hcl") {
		t.Errorf("expected '+++ b/foo.hcl' header, got:\n%s", out)
	}
	if !strings.Contains(out, "-old") {
		t.Errorf("expected '-old' in diff, got:\n%s", out)
	}
	if !strings.Contains(out, "+new") {
		t.Errorf("expected '+new' in diff, got:\n%s", out)
	}
}

func TestPrintDiffColouring(t *testing.T) {
	if err := termcolor.SetMode(termcolor.ModeAlways); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	buf := captureDiff(t)

	printDiff("foo.hcl", "line1\nold\nline3\n", "line1\nnew\nline3\n")

	out := buf.String()
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI escape sequences in output, got:\n%q", out)
	}
}

func TestColourDiffLinePrefixOrder(t *testing.T) {
	if err := termcolor.SetMode(termcolor.ModeAlways); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	// "+++ a/file" must be styled as a Path header, never as an added-line
	// (which would use Success green).
	header := colourDiffLine("+++ a/file")
	plus := colourDiffLine("+added")

	if header == plus {
		t.Errorf("'+++' header got the same styling as a '+' content line; prefix ordering is wrong:\nheader=%q\nplus=%q", header, plus)
	}

	// Similarly for "---" headers vs "-" content lines.
	removedHeader := colourDiffLine("--- a/file")
	minus := colourDiffLine("-removed")
	if removedHeader == minus {
		t.Errorf("'---' header got the same styling as a '-' content line; prefix ordering is wrong:\nheader=%q\nminus=%q", removedHeader, minus)
	}
}
