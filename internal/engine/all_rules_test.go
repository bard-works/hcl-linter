package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAllRulesLintFile enables every rule and lints a kitchen-sink HCL file.
// Its purpose is registry coverage: every rule's Name/Enabled/Check path
// fires at least once and exercises the engine's rule iteration loop.
func TestAllRulesLintFile(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	existingDep := filepath.Join(tmpDir, "existing-dep")
	if err := os.MkdirAll(existingDep, 0o755); err != nil {
		t.Fatal(err)
	}

	setupTestConfig(t, tmpDir, `rules {
  block_order {
    enabled = true
    order   = ["terraform", "locals", "dependency", "inputs"]
  }

  array_format {
    enabled             = true
    multiline_threshold = 2
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }

  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["dependency", "include"]
  }

  duplicates {
    enabled = true
    blocks  = ["dependency", "locals"]
  }

  required_fields {
    include {
      expose = true
    }
  }

  required_blocks {
    required {
      type  = "terraform"
      count = "once"
      error = "missing terraform block"
    }
  }

  dependency_paths {
    enabled = true
  }

  include_paths {
    enabled = true
  }

  remote_state {
    enabled         = true
    require_backend = true
  }

  hcl_functions {
    enabled                       = true
    find_in_parent_folders_exists = true
    get_env_has_default           = true
  }

  terraform_block {
    enabled               = true
    source_required       = true
    version_format        = true
    extra_arguments_valid = true
    no_deprecated_fields  = true
  }

  key_value {
    enabled  = true
    key_case = "snake_case"
    disallowed = ["secret"]
  }

  count_for_each {
    enabled                = true
    warn_on_count_zero     = true
    warn_on_empty_for_each = true
    warn_on_conflict       = true
  }

  dependency_outputs {
    enabled = true
  }
}
`)

	loader := newTestLoader(t, tmpDir)
	eng := New(loader)

	target := createHCLFile(t, tmpDir, "terragrunt.hcl", `terraform {
  source  = "./module"
  version = "not-semver"
  remote_state {
    config { bucket = "b" }
  }
}

dependency "existing-dep" {
  config_path = "existing-dep"
}

dependency "missing-dep" {
  config_path = "does-not-exist"
}

dependency "existing-dep" {
  config_path = "existing-dep"
}

include "Bad-Name" {
  path = "does-not-exist.hcl"
}

locals {
  goodKey = "x"
  secret  = "leak"
  env     = get_env("ENV")
}

inputs = {
  a = "x"
  b = "y"
  c = "z"
}
`)

	result, err := eng.LintFile(target)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	if len(result.Issues) == 0 {
		t.Fatal("expected issues from kitchen-sink file, got none")
	}

	// Sanity: rule IDs we expect to fire given the target above. These are
	// the IDs in issue.Rule - a mix of rule-names and sub-check IDs.
	wantRules := []string{
		"dependency_path_exists",
		"include_path_exists",
		"remote_state_backend_required",
		"terraform_version_format",
		"get_env_has_default",
		"name_validation",
		"duplicates",
		"disallowed_keys",
	}
	got := map[string]bool{}
	for _, issue := range result.Issues {
		got[issue.Rule] = true
	}
	for _, want := range wantRules {
		if !got[want] {
			t.Errorf("expected rule %q to fire; fired rules: %v", want, got)
		}
	}
}

// TestAllFixableRulesFixFile enables every fixable rule and runs FixFile on a
// file with violations for each, exercising the Fix* branches in engine.go.
func TestAllFixableRulesFixFile(t *testing.T) {
	tmpDir := createTestConfigDir(t)

	setupTestConfig(t, tmpDir, `rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "inputs"]
  }

  array_format {
    enabled             = true
    multiline_threshold = 2
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }

  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["include"]
  }

  required_fields {
    include {
      expose = true
    }
  }
}
`)

	loader := newTestLoader(t, tmpDir)
	eng := New(loader)

	target := createHCLFile(t, tmpDir, "terragrunt.hcl",
		"locals {\n\n\n  x = 1\n}\n\ninclude \"Bad-Name\" {}\n\ninputs = { a = \"x\", b = \"y\", c = \"z\" }\n",
	)

	result, err := eng.FixFile(target)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}
	if result.Changes == 0 {
		t.Error("expected changes from fixable violations, got 0")
	}
}
