package main

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestExplainUnknownRule(t *testing.T) {
	err := printExplainDetail("does_not_exist")
	if err == nil {
		t.Fatal("expected error for unknown rule, got nil")
	}
	if !strings.Contains(err.Error(), "does_not_exist") {
		t.Errorf("error %q should mention rule name", err.Error())
	}
}

func TestExplainKnownRule(t *testing.T) {
	err := printExplainDetail("block_order")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExplainNoArgs(_ *testing.T) {
	// printExplainTable should not panic and returns no error
	printExplainTable()
}

func TestDocAllRules(t *testing.T) {
	for _, r := range rules.DefaultRegistry().All() {
		doc := r.Doc()
		if doc.Summary == "" {
			t.Errorf("rule %q has empty Doc().Summary", r.Name())
		}
		if doc.Severity == "" {
			t.Errorf("rule %q has empty Doc().Severity", r.Name())
		}
		if doc.ConfigBlock == "" {
			t.Errorf("rule %q has empty Doc().ConfigBlock", r.Name())
		}
	}
}
