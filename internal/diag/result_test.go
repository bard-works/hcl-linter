package diag

import (
	"strings"
	"testing"
)

func TestResultHasErrors(t *testing.T) {
	tests := []struct {
		name     string
		issues   []Issue
		expected bool
	}{
		{"empty", nil, false},
		{"only warnings", []Issue{{Severity: SeverityWarning}}, false},
		{"one error", []Issue{{Severity: SeverityError}}, true},
		{"mixed", []Issue{{Severity: SeverityWarning}, {Severity: SeverityError}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{Issues: tt.issues}
			if got := r.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResultSummary(t *testing.T) {
	r := &Result{
		File: "/some/dir/terragrunt.hcl",
		Issues: []Issue{
			{Severity: SeverityError},
			{Severity: SeverityWarning},
		},
	}
	got := r.Summary()
	if !strings.Contains(got, "terragrunt.hcl") {
		t.Errorf("Summary %q should contain filename", got)
	}
	if !strings.Contains(got, "2") {
		t.Errorf("Summary %q should report 2 issues", got)
	}
}
