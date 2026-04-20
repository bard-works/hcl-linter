package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/rules"
)

// FixArrays's multiline-input branch (collectMultilineArray / needsMultilineFix /
// fixMultilineArray) fires when the input is already a multi-line array but the
// items are missing trailing commas — the fixer rewrites it with trailing commas.
func TestFixArraysMultilineInputMissingCommas(t *testing.T) {
	input := `list = [
  "a"
  "b"
  "c"
]
`
	got, changes := rules.FixArrays(input, false)
	if changes == 0 {
		t.Fatalf("expected changes for malformed multiline array, got none. output:\n%s", got)
	}
	// After fix, each item line should end with a comma.
	lines := strings.Split(got, "\n")
	commaCount := 0
	for _, line := range lines {
		if strings.HasSuffix(strings.TrimSpace(line), ",") {
			commaCount++
		}
	}
	if commaCount != 3 {
		t.Errorf("expected 3 items with trailing commas, got %d. output:\n%s", commaCount, got)
	}
}

func TestFixArraysMultilineInputAlreadyCorrect(t *testing.T) {
	input := `list = [
  "a",
  "b",
]
`
	got, changes := rules.FixArrays(input, false)
	if changes != 0 {
		t.Errorf("expected no changes for well-formed multiline array, got %d. output:\n%s", changes, got)
	}
}

func TestFixArraysMultilineInputSingleItem(t *testing.T) {
	// Single-item multiline array should be left alone (threshold is 2+).
	input := `list = [
  "only"
]
`
	_, changes := rules.FixArrays(input, false)
	if changes != 0 {
		t.Errorf("expected no changes for single-item multiline array, got %d", changes)
	}
}

func TestFixArraysMultilineWithSort(t *testing.T) {
	input := `list = [
  "c"
  "a"
  "b"
]
`
	got, _ := rules.FixArrays(input, true)
	// With sort enabled, items should appear in alphabetical order.
	idxA := strings.Index(got, `"a"`)
	idxB := strings.Index(got, `"b"`)
	idxC := strings.Index(got, `"c"`)
	if idxA >= idxB || idxB >= idxC {
		t.Errorf("expected sorted order a,b,c in output:\n%s", got)
	}
}
