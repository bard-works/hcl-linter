# Rules Reference

This document describes every rule shipped with `hcl-linter`, the
configuration fields each accepts, and the rule IDs and default severities
emitted when a violation is found.

The core rules (block order, formatting, naming, required fields, blank
lines, required blocks, key-value validation, count/for_each) work against
any HCL 2 file. The Terragrunt- and Terraform-specific rules (dependency
paths, include paths, remote state, hcl functions, terraform block,
dependency outputs) ship as built-ins.

## Contents

1. [Summary table](#summary-table)
2. [`block_order` - Block Order](#1-block-order-block_order)
3. [`array_format` - Array Format](#2-array-format-array_format)
4. [`blank_lines` - Blank Lines](#3-blank-lines-blank_lines)
5. [`name_validation` - Name Validation](#4-name-validation-name_validation)
6. [`duplicates` - Duplicate Detection](#5-duplicate-detection-duplicates)
7. [`required_fields` - Required Fields](#6-required-fields-required_fields)
8. [`required_blocks` - Required Blocks](#7-required-blocks-required_blocks)
9. [`dependency_paths` - Dependency Paths](#8-dependency-paths-dependency_paths)
10. [`include_paths` - Include Paths](#9-include-paths-include_paths)
11. [`remote_state` - Remote State](#10-remote-state-remote_state)
12. [`hcl_functions` - HCL Functions](#11-hcl-functions-hcl_functions)
13. [`terraform_block` - Terraform Block](#12-terraform-block-terraform_block)
14. [`key_value` - Key-Value Validation](#13-key-value-validation-key_value)
15. [`count_for_each` - Count / ForEach](#14-countforeach-validation-count_for_each)
16. [`dependency_outputs` - Dependency Outputs](#15-dependency-output-validation-dependency_outputs)

## Summary table

| Config block         | Rule ID(s) emitted                                                                                                     | Default severity |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------- | ---------------- |
| `block_order`        | `block_order`                                                                                                          | error            |
| `array_format`       | `array_format`                                                                                                         | warning          |
| `blank_lines`        | _(fix-only - no Check phase, emits no issues)_                                                                         | -                |
| `name_validation`    | `name_validation`                                                                                                      | error            |
| `duplicates`         | `duplicates`                                                                                                           | error            |
| `required_fields`    | `required_fields`                                                                                                      | error            |
| `required_blocks`    | `required_blocks`                                                                                                      | error            |
| `dependency_paths`   | `dependency_path_exists`                                                                                               | error            |
| `include_paths`      | `include_path_exists`                                                                                                  | error            |
| `remote_state`       | `remote_state_backend_required`                                                                                        | error            |
| `hcl_functions`      | `find_in_parent_folders_exists` (error), `get_env_has_default` (warning)                                               | mixed            |
| `terraform_block`    | `terraform_source_required` (error), `terraform_version_format`, `terraform_extra_arguments_valid`, `terraform_deprecated_fields` (all warning) | mixed            |
| `key_value`          | `key_case`, `disallowed_keys` (error), `value_pattern` (warning)                                                       | mixed            |
| `count_for_each`     | `count_zero` (warning), `empty_for_each` (warning), `count_for_each_conflict` (error)                                  | mixed            |
| `dependency_outputs` | `dependency_outputs`                                                                                                   | warning          |

---

### 1. Block Order (`block_order`)

**Purpose:** Enforce consistent ordering of top-level blocks.

**Rule ID:** `block_order` - **Severity:** error

**Behavior:**

- Order of listed blocks is enforced
- Unlisted blocks (e.g. `outputs`, custom blocks) are placed after the last listed block
- Supports nested block ordering via `nested_order`

**Configuration:**

```hcl
rules {
  block_order {
    enabled      = true
    order        = ["include", "locals", "terraform", "dependency", "inputs"]
    nested_order = {
      terraform = ["before_hooks", "after_hooks"]
    }
  }
}
```

- `nested_order` (optional): Map of parent block types to their nested block ordering rules
- Use dot notation in comments for documentation (e.g. `terraform.before_hooks`)

**Example violation:**

```hcl
# Wrong order
locals {}
include "root" {}
terraform {}

# Correct order
include "root" {}
locals {}
terraform {}
```

**Nested block ordering example:**

```hcl
# Wrong nested order
terraform {
  after_hooks {
    exec {
      command = "echo after"
    }
  }
  before_hooks {
    exec {
      command = "echo before"
    }
  }
}

# Correct nested order
terraform {
  before_hooks {
    exec {
      command = "echo before"
    }
  }
  after_hooks {
    exec {
      command = "echo after"
    }
  }
}
```

### 2. Array Format (`array_format`)

**Purpose:** Normalize array formatting.

**Rule ID:** `array_format` - **Severity:** warning

**Rules:**

- 1 item: inline `actions = ["a"]`
- 2+ items: multiline with trailing comma

```hcl
actions = [
  "a",
  "b",
]
```

**Edge cases:**

- Arrays with non-quoted items (e.g. `dependency.x.outputs.y`) are left unchanged
- Empty arrays remain unchanged

### 3. Blank Lines (`blank_lines`)

**Purpose:** Clean up blank lines within blocks for consistent formatting.

**Rule ID:** _(fix-only; no issues emitted during lint/check)_

**Configuration:**

```hcl
rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}
```

**Behavior:**

- `within_blocks: true` - Cleans blank lines inside object attributes (`inputs = {}`) and top-level blocks (`terraform {}`)
- Removes blank lines at the beginning (after `{`) and end (before `}`) of blocks
- Reduces consecutive duplicate blank lines to a single blank line
- Single blank lines between attributes are preserved
- Blank lines between top-level blocks are preserved
- Nested blocks (e.g. `before_hook` inside `terraform`) are also cleaned

**Example:**

```hcl
# Before
inputs = {

  name = "test"


  options = "none"

}

# After
inputs = {
  name = "test"

  options = "none"
}
```

### 4. Name Validation (`name_validation`)

**Purpose:** Ensure consistent naming conventions.

**Rule ID:** `name_validation` - **Severity:** error

**Checks:**

- Include/dependency block labels must match regex pattern
- Default: `^[a-z][a-z0-9_]*$` (no hyphens)

**Example violation:**

```hcl
# Wrong
include "my-vpc" {}

# Correct
include "my_vpc" {}
```

### 5. Duplicate Detection (`duplicates`)

**Purpose:** Detect duplicate blocks.

**Rule ID:** `duplicates` - **Severity:** error

**Checks:**

- Duplicate `dependency` block labels
- Duplicate `include` block labels

**Example violation:**

```hcl
# Duplicate dependencies
dependency "vpc" {}
dependency "vpc" {}  # ERROR: duplicate
```

### 6. Required Fields (`required_fields`)

**Purpose:** Enforce required attributes per block type.

**Rule ID:** `required_fields` - **Severity:** error

**Configuration:**

```hcl
rules {
  required_fields {
    include {
      expose = true
    }
  }
}
```

**Checks:**

- When block exists, required attributes must be present
- Boolean `true` means attribute must exist with any value

### 7. Required Blocks (`required_blocks`)

**Purpose:** Enforce that certain block types must exist in the file.

**Rule ID:** `required_blocks` - **Severity:** error

**Configuration:**

```hcl
rules {
  required_blocks {
    required {
      type  = "terraform"
      count = "once"
      error = "missing terraform block"
    }
  }
}
```

**Supported count values:**

- `once` - Block must appear exactly once

**Checks:**

- Reports error if required block is missing or appears more than once
- Error message is customizable per block type
- File-pattern based (applies only to files matching the config)

**Example:**

```hcl
# With config requiring terraform block:
# OK
terraform {}

# ERROR: missing terraform block
include "root" {}
locals {}
```

### 8. Dependency Paths (`dependency_paths`)

**Purpose:** Validate that `config_path` attributes on `dependency` blocks
resolve to existing directories. Targets the Terragrunt `dependency` block
shape.

**Rule ID:** `dependency_path_exists` - **Severity:** error

**Configuration:**

```hcl
rules {
  dependency_paths {
    enabled = true
  }
}
```

**Example violation:**

```hcl
dependency "vpc" {
  config_path = "../non-existent-vpc"  # ERROR: directory does not exist
}
```

### 9. Include Paths (`include_paths`)

**Purpose:** Validate that `path` attributes on `include` blocks resolve to
existing files or directories. Function-call values (e.g.
`find_in_parent_folders()`) are skipped.

**Rule ID:** `include_path_exists` - **Severity:** error

**Configuration:**

```hcl
rules {
  include_paths {
    enabled = true
  }
}
```

**Example violation:**

```hcl
include "root" {
  path = "non-existent/parent.hcl"  # ERROR: file does not exist
}
```

### 10. Remote State (`remote_state`)

**Purpose:** Validate `remote_state { }` blocks nested inside the top-level
`terraform { }` block. When `require_backend = true`, the `backend`
attribute must be set and non-empty.

**Rule ID:** `remote_state_backend_required` - **Severity:** error

**Configuration:**

```hcl
rules {
  remote_state {
    enabled         = true
    require_backend = true
  }
}
```

**Example violation:**

```hcl
terraform {
  remote_state {
    # ERROR: missing required 'backend' attribute
    config {
      bucket = "my-bucket"
    }
  }
}
```

### 11. HCL Functions (`hcl_functions`)

**Purpose:** Validate common HCL function calls. Currently recognizes
Terragrunt's `find_in_parent_folders` and `get_env`.

**Rule IDs and severities:**

| Rule ID                         | Severity | Emitted when                                                              |
| ------------------------------- | -------- | ------------------------------------------------------------------------- |
| `find_in_parent_folders_exists` | error    | The file passed to `find_in_parent_folders(...)` is not found             |
| `get_env_has_default`           | warning  | `get_env("X")` is called without a default (`get_env("X", "fallback")`)   |

**Configuration:**

```hcl
rules {
  hcl_functions {
    enabled                       = true
    find_in_parent_folders_exists = true
    get_env_has_default           = true
  }
}
```

**Example violations:**

```hcl
# find_in_parent_folders_exists violation
inputs = {
  config = find_in_parent_folders("missing-file.hcl")  # ERROR: file not found
}

# get_env_has_default violation
locals {
  env = get_env("ENVIRONMENT")  # WARNING: should have default
}

# Correct usage
locals {
  env = get_env("ENVIRONMENT", "dev")
}
```

### 12. Terraform Block (`terraform_block`)

**Purpose:** Validate Terragrunt's `terraform` block configuration.

**Rule IDs and severities:**

| Rule ID                           | Severity | Emitted when                                                      |
| --------------------------------- | -------- | ----------------------------------------------------------------- |
| `terraform_source_required`       | error    | `terraform { }` is missing a `source` attribute                   |
| `terraform_version_format`        | warning  | `version` does not match the expected constraint format           |
| `terraform_extra_arguments_valid` | warning  | `extra_arguments` block missing a label or has no arguments       |
| `terraform_deprecated_fields`     | warning  | Uses deprecated block types (`before_hook`, `after_hook`, nested `terraform`) |

**Configuration:**

```hcl
rules {
  terraform_block {
    enabled               = true
    source_required       = true
    version_format        = true
    extra_arguments_valid = true
    no_deprecated_fields  = true
  }
}
```

**Example violations:**

```hcl
# source_required violation
terraform {
  # ERROR: missing required 'source' attribute
}

# version_format violation
terraform {
  version = "1.2.3"  # WARNING: may not match expected format
}

# extra_arguments_valid violations
terraform {
  extra_arguments {}  # ERROR: missing name attribute

  extra_arguments "example" {}  # ERROR: missing arguments or nested blocks
}

# no_deprecated_fields violations
terraform {
  before_hook {  # ERROR: use 'before_hooks' instead
    commands = ["echo hello"]
  }

  after_hook {  # ERROR: use 'after_hooks' instead
    commands = ["echo hello"]
  }

  terraform {  # ERROR: use 'source' instead
    source = "./module"
  }
}

# Correct usage
terraform {
  source  = "./module"
  version = ">= 1.0.0"

  before_hooks {
    commands = ["echo hello"]
  }

  after_hooks {
    commands = ["echo hello"]
  }

  extra_arguments "example" {
    arguments = ["-var", "foo=bar"]
  }
}
```

### 13. Key-Value Validation (`key_value`)

**Purpose:** Enforce attribute naming conventions, validate values against
patterns, and blocklist certain keys.

**Rule IDs and severities:**

| Rule ID           | Severity | Emitted when                                                        |
| ----------------- | -------- | ------------------------------------------------------------------- |
| `key_case`        | error    | An attribute name violates the configured case convention           |
| `disallowed_keys` | error    | An attribute name appears in the `disallowed` list                  |
| `value_pattern`   | warning  | An attribute value fails the configured regex for that attribute    |

**Configuration:**

```hcl
rules {
  key_value {
    enabled   = true
    key_case  = "snake_case"
    value_pattern = {
      region = "^us-[a-z]+-[0-9]+$"
    }
    disallowed = ["secret", "password"]
  }
}
```

**Supported case values:**

- `camelCase` - `^[a-z][a-zA-Z0-9]*$`
- `snake_case` - `^[a-z][a-z0-9_]*$`
- `kebab-case` - `^[a-z][a-z0-9-]*$`

**Example violations:**

```hcl
# key_case violation (snake_case expected)
locals {
  myVar  = "test"  # ERROR: should be snake_case
  my-var = "test"  # ERROR: should be snake_case
}

# value_pattern violation
locals {
  region = "invalid"  # WARNING: does not match pattern ^us-[a-z]+-[0-9]+$
}

# disallowed_keys violation
locals {
  secret   = "abc"  # ERROR: attribute is not allowed
  password = "123"  # ERROR: attribute is not allowed
}

# Correct usage
locals {
  my_var  = "test"        # snake_case ✓
  region  = "us-east-1"   # matches pattern ✓
  api_key = "abc"         # allowed key ✓
}
```

### 14. Count/ForEach Validation (`count_for_each`)

**Purpose:** Detect potential issues with count and for_each expressions.

**Rule IDs and severities:**

| Rule ID                   | Severity | Emitted when                                      |
| ------------------------- | -------- | ------------------------------------------------- |
| `count_zero`              | warning  | `count = 0` (resource will not be created)        |
| `empty_for_each`          | warning  | `for_each = {}` (resource will not be created)    |
| `count_for_each_conflict` | error    | Both `count` and `for_each` used on the same block |

**Configuration:**

```hcl
rules {
  count_for_each {
    enabled                = true
    warn_on_count_zero     = true
    warn_on_empty_for_each = true
    warn_on_conflict       = true
  }
}
```

**Example violations:**

```hcl
# count_zero violation
resource "aws_instance" "test" {
  count = 0  # WARNING: resource will not be created
}

# empty_for_each violation
resource "aws_instance" "test" {
  for_each = {}  # WARNING: resource will not be created
}

# count_for_each_conflict violation
resource "aws_instance" "test" {
  count    = 1  # ERROR: cannot use both count and for_each
  for_each = {}
}
```

### 15. Dependency Output Validation (`dependency_outputs`)

**Purpose:** Validate `dependency.*.outputs.*` references by walking the
dependency chain.

**Rule ID:** `dependency_outputs` - **Severity:** warning

**Configuration:**

```hcl
rules {
  dependency_outputs {
    enabled = true
  }
}
```

**Behavior:**

1. **Dependency resolution:**
   - Parse `dependency` blocks to get `config_path`
   - Support relative paths (e.g. `../vpc`, `vpc`)
   - Skip paths with variables/expressions (shown in verbose mode)
   - Detect circular dependencies to avoid infinite loops

2. **Output validation:**
   - Parse `output` blocks from `.tf` files in dependency module
   - Validate output name exists
   - Warn if output doesn't exist

3. **Mock outputs** (for development):
   - Support `.mock-outputs.json` in dependency module directory
   - Format: `{"outputs": {"vpc_id": {"value": "vpc-123", "type": "string"}}}`
   - Optional - if not present, only validate against `.tf` files

4. **Error handling:**
   - If output doesn't exist = warning
   - If dependency path doesn't exist = warning (validation skipped)
   - No terraform/terragrunt execution - purely static analysis

**Example violations:**

```hcl
# In your terragrunt.hcl
dependency "vpc" {
  config_path = "../vpc"
}

# Reference outputs
inputs = {
  vpc_id = dependency.vpc.outputs.vpc_id   # ✓ validated
  fake   = dependency.vpc.outputs.fake_id  # ✗ not defined in ../vpc/outputs.tf
}
```
