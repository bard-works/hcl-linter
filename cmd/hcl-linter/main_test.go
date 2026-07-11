package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		filename string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "terragrunt.hcl",
			filename: "terragrunt.hcl",
			expected: true,
		},
		{
			name:     "exact no match",
			pattern:  "terragrunt.hcl",
			filename: "service.hcl",
			expected: false,
		},
		{
			name:     "suffix glob",
			pattern:  "*.hcl",
			filename: "terragrunt.hcl",
			expected: true,
		},
		{
			name:     "suffix glob no match",
			pattern:  "*.hcl",
			filename: "service.tf",
			expected: false,
		},
		{
			name:     "prefix glob",
			pattern:  "terragrunt*",
			filename: "terragrunt.hcl",
			expected: true,
		},
		{
			name:     "prefix glob no match",
			pattern:  "terragrunt*",
			filename: "service.hcl",
			expected: false,
		},
		{
			name:     "middle glob",
			pattern:  "*.tf",
			filename: "main.tf",
			expected: true,
		},
		{
			name:     "no glob exact match",
			pattern:  "root",
			filename: "root.hcl",
			expected: false,
		},
		{
			// Prior hand-rolled matchGlob degraded to exact-string equality
			// for any pattern with 2+ '*' — this must now match correctly.
			name:     "multiple wildcards",
			pattern:  "*env*.hcl",
			filename: "prod-env-x.hcl",
			expected: true,
		},
		{
			name:     "three wildcards",
			pattern:  "a*b*c",
			filename: "axbxc",
			expected: true,
		},
		{
			name:     "single char wildcard",
			pattern:  "terragrunt.?cl",
			filename: "terragrunt.hcl",
			expected: true,
		},
		{
			name:     "character class",
			pattern:  "service.[th]f",
			filename: "service.tf",
			expected: true,
		},
		{
			name:     "malformed pattern does not panic",
			pattern:  "[",
			filename: "anything",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.filename)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.filename, result, tt.expected)
			}
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	a := newTestApp()

	// Test with no filters
	t.Run("no filters matches all", func(t *testing.T) {
		if !a.matchesFilter("anyfile.hcl") {
			t.Error("expected matchesFilter to return true with no filters")
		}
	})

	// Test with single filter
	a.filter = []string{"*.hcl"}

	t.Run("single filter match", func(t *testing.T) {
		if !a.matchesFilter("file.hcl") {
			t.Error("expected matchesFilter to return true for *.hcl matching file.hcl")
		}
	})

	t.Run("single filter no match", func(t *testing.T) {
		if a.matchesFilter("file.tf") {
			t.Error("expected matchesFilter to return false for *.hcl not matching file.tf")
		}
	})

	// Test with multiple filters (OR logic)
	a.filter = []string{"*.hcl", "*.tf"}

	t.Run("multiple filters first matches", func(t *testing.T) {
		if !a.matchesFilter("file.hcl") {
			t.Error("expected matchesFilter to return true")
		}
	})

	t.Run("multiple filters second matches", func(t *testing.T) {
		if !a.matchesFilter("file.tf") {
			t.Error("expected matchesFilter to return true")
		}
	})

	t.Run("multiple filters neither matches", func(t *testing.T) {
		if a.matchesFilter("file.json") {
			t.Error("expected matchesFilter to return false")
		}
	})
}

func TestFindHCLFiles(t *testing.T) {
	// This test would require creating temp directories with HCL files
	// For now, just test the function exists and can be called
	t.Run("function exists", func(_ *testing.T) {
		result := newTestApp().findHCLFiles("/tmp")
		// Result might be empty or contain files, just verify it doesn't panic
		_ = result
	})
}

func TestFilterFiles(t *testing.T) {
	// Test with mock files
	testFiles := []string{
		"/path/to/terragrunt.hcl",
		"/path/to/service.hcl",
		"/path/to/main.tf",
		"/path/to/vars.json",
	}

	// Test with *.hcl filter
	a := newTestApp()
	a.filter = []string{"*.hcl"}

	result := a.filterFiles(testFiles)
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d", len(result))
	}

	// Test with multiple filters
	a.filter = []string{"terragrunt.hcl", "*.tf"}
	result = a.filterFiles(testFiles)
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d", len(result))
	}
}

func TestFilterFilesMatchedListingGatedByVerbose(t *testing.T) {
	testFiles := []string{"/path/to/terragrunt.hcl"}

	a := newTestApp()
	var buf bytes.Buffer
	a.errOut = &buf
	a.filter = []string{"*.hcl"}
	a.verbose = false
	a.filterFiles(testFiles)
	if strings.Contains(buf.String(), "Matched files:") {
		t.Error("expected no 'Matched files:' listing without --verbose")
	}

	buf.Reset()
	a.verbose = true
	a.filterFiles(testFiles)
	if !strings.Contains(buf.String(), "Matched files:") {
		t.Error("expected 'Matched files:' listing with --verbose")
	}
}
