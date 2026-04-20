package main

import (
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

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		pattern  string
		expected bool
	}{
		{
			name:     "prefix wildcard",
			filename: "terragrunt.hcl",
			pattern:  "terragrunt*",
			expected: true,
		},
		{
			name:     "suffix wildcard",
			filename: "main.tf",
			pattern:  "*.tf",
			expected: true,
		},
		{
			name:     "both wildcards",
			filename: "service.hcl",
			pattern:  "*.hcl",
			expected: true,
		},
		{
			name:     "no match",
			filename: "other.json",
			pattern:  "*.hcl",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchGlob(tt.filename, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.filename, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	flagFilter = nil
	defer func() { flagFilter = nil }()

	// Test with no filters
	t.Run("no filters matches all", func(t *testing.T) {
		if !matchesFilter("anyfile.hcl") {
			t.Error("expected matchesFilter to return true with no filters")
		}
	})

	// Test with single filter
	flagFilter = []string{"*.hcl"}
	defer func() { flagFilter = nil }()

	t.Run("single filter match", func(t *testing.T) {
		if !matchesFilter("file.hcl") {
			t.Error("expected matchesFilter to return true for *.hcl matching file.hcl")
		}
	})

	t.Run("single filter no match", func(t *testing.T) {
		if matchesFilter("file.tf") {
			t.Error("expected matchesFilter to return false for *.hcl not matching file.tf")
		}
	})

	// Test with multiple filters (OR logic)
	flagFilter = []string{"*.hcl", "*.tf"}

	t.Run("multiple filters first matches", func(t *testing.T) {
		if !matchesFilter("file.hcl") {
			t.Error("expected matchesFilter to return true")
		}
	})

	t.Run("multiple filters second matches", func(t *testing.T) {
		if !matchesFilter("file.tf") {
			t.Error("expected matchesFilter to return true")
		}
	})

	t.Run("multiple filters neither matches", func(t *testing.T) {
		if matchesFilter("file.json") {
			t.Error("expected matchesFilter to return false")
		}
	})
}

func TestFindHCLFiles(t *testing.T) {
	// This test would require creating temp directories with HCL files
	// For now, just test the function exists and can be called
	t.Run("function exists", func(t *testing.T) {
		result := findHCLFiles("/tmp")
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
	flagFilter = []string{"*.hcl"}
	defer func() { flagFilter = nil }()

	result := filterFiles(testFiles)
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d", len(result))
	}

	// Test with multiple filters
	flagFilter = []string{"terragrunt.hcl", "*.tf"}
	result = filterFiles(testFiles)
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d", len(result))
	}
}
