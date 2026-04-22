package rules_test

import (
	"testing"

	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestDefaultRegistryContainsAllRules(t *testing.T) {
	all := rules.DefaultRegistry().All()
	if len(all) != len(allRegisteredRules()) {
		t.Errorf("DefaultRegistry has %d rules, want %d", len(all), len(allRegisteredRules()))
	}
}

func TestEveryRulePriorityAndDoc(t *testing.T) {
	validPriorities := map[int]bool{
		rules.PriorityStructure: true,
		rules.PrioritySemantic:  true,
		rules.PriorityFormat:    true,
		rules.PriorityFinal:     true,
	}

	for _, r := range rules.DefaultRegistry().All() {
		p := r.Priority()
		if !validPriorities[p] {
			t.Errorf("rule %q: Priority()=%d not one of the defined constants", r.Name(), p)
		}

		doc := r.Doc()
		if doc.Summary == "" {
			t.Errorf("rule %q: Doc().Summary is empty", r.Name())
		}
		if doc.Severity != "error" && doc.Severity != "warning" {
			t.Errorf("rule %q: Doc().Severity=%q, want error|warning", r.Name(), doc.Severity)
		}
		if doc.ConfigBlock == "" {
			t.Errorf("rule %q: Doc().ConfigBlock is empty", r.Name())
		}
		if len(doc.ConfigFields) == 0 {
			t.Errorf("rule %q: Doc().ConfigFields is empty", r.Name())
		}
		for _, f := range doc.ConfigFields {
			if f.Name == "" {
				t.Errorf("rule %q: ConfigField has empty Name", r.Name())
			}
			if f.Type == "" {
				t.Errorf("rule %q: ConfigField %q has empty Type", r.Name(), f.Name)
			}
		}
	}
}

func TestRegistrySortedOrdersByPriorityAscending(t *testing.T) {
	sorted := rules.DefaultRegistry().Sorted()
	if len(sorted) == 0 {
		t.Fatal("Sorted() returned empty slice")
	}
	prev := sorted[0].Priority()
	for _, r := range sorted[1:] {
		p := r.Priority()
		if p < prev {
			t.Errorf("Sorted() out of order: %d came after %d", p, prev)
		}
		prev = p
	}
}

func TestBlankLinesRuleCheckReturnsNil(t *testing.T) {
	if issues := (rules.BlankLinesRule{}).Check(nil); issues != nil {
		t.Errorf("BlankLinesRule.Check should return nil, got %v", issues)
	}
}
