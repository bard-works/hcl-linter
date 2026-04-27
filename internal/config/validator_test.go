package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertIssues(t *testing.T, issues []ValidationIssue, wantCount int, wantSubstrings ...string) {
	t.Helper()
	if len(issues) != wantCount {
		t.Errorf("expected %d issue(s), got %d:", wantCount, len(issues))
		for _, i := range issues {
			t.Errorf("  - %s", i)
		}
		return
	}
	for _, sub := range wantSubstrings {
		found := false
		for _, issue := range issues {
			if contains(issue.Message, sub) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no issue contains %q; got: %v", sub, issues)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

// --- Unknown rule blocks ---

func TestValidateUnknownRuleBlock(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "terragrunt.hcl", `rules {
  blokc_order {
    enabled = true
    order   = ["include", "locals"]
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 1, "blokc_order", "unknown")
}

func TestValidateMultipleUnknownBlocks(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  typo_one {}
  typo_two {}
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 2)
}

func TestValidateAllKnownBlocksClean(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["include"]
  }
  array_format { enabled = true }
  blank_lines  { enabled = true; within_blocks = true }
  duplicates   { enabled = true }
  terragrunt   { enabled = true }
  terragrunt_functions { enabled = true }
  terraform_block      { enabled = true }
  count_for_each       { enabled = true }
  dependency_outputs   { enabled = true }
  key_value {
    enabled  = true
    key_case = "snake_case"
  }
  required_fields {
    include { expose = true }
  }
  required_blocks {
    required {
      type  = "terraform"
      count = "once"
      error = "missing"
    }
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 0)
}

// --- block_order required fields ---

func TestValidateBlockOrderEnabledEmptyOrder(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  block_order {
    enabled = true
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 1, "block_order", "order")
}

func TestValidateBlockOrderDisabledEmptyOrderOk(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  block_order {
    enabled = false
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 0)
}

// --- name_validation required fields ---

func TestValidateNameValidationEnabledNoPattern(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  name_validation {
    enabled = true
    blocks  = ["include"]
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 1, "name_validation", "pattern")
}

func TestValidateNameValidationWithPatternOk(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  name_validation {
    enabled = true
    pattern = "^[a-z]+$"
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 0)
}

// --- required_blocks required fields ---

func TestValidateRequiredBlocksNoEntries(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  required_blocks {}
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 1, "required_blocks", "no 'required' entries")
}

func TestValidateRequiredBlocksWithEntriesOk(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  required_blocks {
    required {
      type  = "terraform"
      count = "once"
      error = "missing"
    }
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 0)
}

// --- key_value required fields ---

func TestValidateKeyValueEnabledNoChecks(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  key_value {
    enabled = true
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 1, "key_value")
}

func TestValidateKeyValueWithKeyCaseOk(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  key_value {
    enabled  = true
    key_case = "snake_case"
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 0)
}

func TestValidateKeyValueWithDisallowedOk(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  key_value {
    enabled    = true
    disallowed = ["password"]
  }
}`)

	issues := ValidateConfigFile(path)
	assertIssues(t, issues, 0)
}

// --- ValidateDir ---

func TestValidateDirMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "default.hcl", `rules {
  block_order {
    enabled = true
  }
}`)
	writeConfig(t, dir, "service.hcl", `rules {
  typo_rule {}
}`)
	writeConfig(t, dir, "clean.hcl", `rules {
  block_order {
    enabled = true
    order   = ["include"]
  }
}`)

	issues := ValidateDir(dir)
	if len(issues) != 2 {
		t.Errorf("expected 2 issues (1 per bad file), got %d:", len(issues))
		for _, i := range issues {
			t.Errorf("  - %s", i)
		}
	}
}

func TestValidateDirEmptyDir(t *testing.T) {
	dir := t.TempDir()
	issues := ValidateDir(dir)
	if len(issues) != 0 {
		t.Errorf("expected no issues for empty dir, got %d", len(issues))
	}
}

func TestValidateDirNonExistent(t *testing.T) {
	issues := ValidateDir("/does/not/exist")
	if len(issues) != 1 {
		t.Errorf("expected 1 error for non-existent dir, got %d", len(issues))
	}
}

// --- multiple issues in one file ---

func TestValidateMultipleIssuesSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "default.hcl", `rules {
  bad_rule {}
  block_order { enabled = true }
  name_validation { enabled = true }
}`)

	issues := ValidateConfigFile(path)
	// unknown block + block_order empty order + name_validation no pattern = 3
	if len(issues) != 3 {
		t.Errorf("expected 3 issues, got %d:", len(issues))
		for _, i := range issues {
			t.Errorf("  - %s", i)
		}
	}
}
