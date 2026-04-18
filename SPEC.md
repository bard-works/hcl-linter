# HCL Linter - Specification

A configurable linter for Terragrunt HCL files that enforces consistency standards across large codebases.

## Goals

- Enforce block ordering, formatting, naming, required fields, blank lines, required blocks, and Terragrunt-specific validations
- Configurable rules per filename pattern (terragrunt.hcl, root.hcl, service.hcl)
- Auto-fix capability for formatable issues
- Extensible config system with user-defined configurations
- Fast processing with configurable concurrency

## Configuration

Configuration files in `.hcl-linter/` directory, named after the file pattern they configure:

```
.hcl-linter/
├── default.json       # Base config for all files (optional)
├── terragrunt.json    # Rules for terragrunt.hcl
├── root.json          # Rules for root.hcl
└── service.json       # Rules for service.hcl
```

### Config Source Precedence

Configs are loaded in the following order (first match wins):

| Priority | Source                      | Description                      |
| -------- | --------------------------- | -------------------------------- |
| 1        | CLI `--config-source`       | Explicit path via flag           |
| 2        | Env `HCL_LINTER_CONFIG_DIR` | Environment variable             |
| 3        | `.hcl-linter/` in cwd       | User config in current directory |
| 4        | `.hcl-linter/` in home      | User config in home directory    |
| 5        | Project's `.hcl-linter/`    | Built-in defaults (fallback)     |

**Full override**: User config completely replaces project configs - no merging.

**Applied configs are printed at startup** for transparency.

If falling back to project defaults (no user config found), a warning is displayed.

### Configuration Schema (JSON)

```json
{
  "rules": {
    "block_order": {
      "enabled": true,
      "order": ["include", "locals", "terraform", "dependency", "inputs"]
    },
    "array_format": {
      "enabled": true,
      "multiline_threshold": 2
    },
    "blank_lines": {
      "enabled": true,
      "within_blocks": true
    },
    "name_validation": {
      "enabled": true,
      "pattern": "^[a-z][a-z0-9_]*$",
      "blocks": ["include", "dependency"]
    },
    "duplicates": {
      "enabled": true,
      "blocks": ["locals", "dependency", "include"]
    },
    "required_fields": {
      "include": {
        "expose": true
      }
    },
    "required_blocks": {
      "required": [
        {
          "type": "terraform",
          "count": "once",
          "error": "missing terraform block"
        }
      ]
    },
    "terragrunt": {
      "enabled": true,
      "dependency_path_exists": true,
      "include_path_exists": true,
      "remote_state_config": true
    },
    "terragrunt_functions": {
      "enabled": true,
      "find_in_parent_folders_exists": true,
      "get_env_has_default": true
    },
    "terraform_block": {
      "enabled": true,
      "source_required": true,
      "version_format": true,
      "extra_arguments_valid": true,
      "no_deprecated_fields": true
    },
    "key_value": {
      "enabled": true,
      "key_case": "snake_case",
      "value_pattern": {
        "region": "^us-[a-z]+-[0-9]+$"
      },
      "disallowed": ["secret", "password"]
    },
    "count_for_each": {
      "enabled": true,
      "warn_on_count_zero": true,
      "warn_on_empty_for_each": true,
      "warn_on_conflict": true
    },
    "dependency_outputs": {
      "enabled": true
    }
  }
}
```

## Rules

### 1. Block Order (`block_order`)

**Purpose:** Enforce consistent ordering of top-level blocks.

**Behavior:**

- Order of listed blocks is enforced
- Unlisted blocks (e.g., `outputs`, custom blocks) are placed after the last listed block

**Configuration:**

```json
{
  "block_order": {
    "enabled": true,
    "order": ["include", "locals", "terraform", "dependency", "inputs"],
    "nested_order": {
      "terraform": ["before_hooks", "after_hooks"]
    }
  }
}
```

- `nested_order` (optional): Map of parent block types to their nested block ordering rules
- Use dot notation in comments for documentation (e.g., `terraform.before_hooks`)

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

- Arrays with non-quoted items (e.g., `dependency.x.outputs.y`) are left unchanged
- Empty arrays remain unchanged

### 3. Blank Lines (`blank_lines`)

**Purpose:** Clean up blank lines within blocks for consistent formatting.

**Configuration:**

```json
{
  "blank_lines": {
    "enabled": true,
    "within_blocks": true
  }
}
```

**Behavior:**

- `within_blocks: true` - Cleans blank lines inside object attributes (`inputs = {}`) and top-level blocks (`terraform {}`)
- Removes blank lines at the beginning (after `{`) and end (before `}`) of blocks
- Reduces consecutive duplicate blank lines to a single blank line
- Single blank lines between attributes are preserved
- Blank lines between top-level blocks are preserved
- Nested blocks (e.g., `before_hook` inside `terraform`) are also cleaned

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

**Configuration:**

```json
{
  "required_fields": {
    "include": {
      "expose": true
    }
  }
}
```

**Checks:**

- When block exists, required attributes must be present
- Boolean `true` means attribute must exist with any value

### 7. Required Blocks (`required_blocks`)

**Purpose:** Enforce that certain block types must exist in the file.

**Configuration:**

```json
{
  "required_blocks": {
    "required": [
      {
        "type": "terraform",
        "count": "once",
        "error": "missing terraform block"
      }
    ]
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

### 8. Terragrunt Validation (`terragrunt`)

**Purpose:** Validate Terragrunt-specific configurations.

**Configuration:**

```json
{
  "terragrunt": {
    "enabled": true,
    "dependency_path_exists": true,
    "include_path_exists": true,
    "remote_state_config": true
  }
}
```

**Checks:**

- `dependency_path_exists` - Validates `dependency.config_path` points to an existing directory
- `include_path_exists` - Validates `include.path` exists (function calls like `find_in_parent_folders()` are skipped)
- `remote_state_config` - Validates `terraform.remote_state` has a `backend` attribute

**Example violations:**

```hcl
# dependency_path_exists violation
dependency "vpc" {
  config_path = "../non-existent-vpc"  # ERROR: directory does not exist
}

# include_path_exists violation
include "root" {
  path = "non-existent/parent.hcl"  # ERROR: file does not exist
}

# remote_state_config violation
terraform {
  remote_state {
    # ERROR: missing required 'backend' attribute
    config {
      bucket = "my-bucket"
    }
  }
}
```

### 9. Terragrunt Functions (`terragrunt_functions`)

**Purpose:** Validate Terragrunt function calls.

**Configuration:**

```json
{
  "terragrunt_functions": {
    "enabled": true,
    "find_in_parent_folders_exists": true,
    "get_env_has_default": true
  }
}
```

**Checks:**

- `find_in_parent_folders_exists` - Validates that the file being searched for exists in parent directories
- `get_env_has_default` - Warns when `get_env()` is called without a default value

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

### 10. Terraform Block (`terraform_block`)

**Purpose:** Validate Terragrunt's `terraform` block configuration.

**Configuration:**

```json
{
  "terraform_block": {
    "enabled": true,
    "source_required": true,
    "version_format": true,
    "extra_arguments_valid": true,
    "no_deprecated_fields": true
  }
}
```

**Checks:**

- `source_required` - Ensures the `terraform` block has a `source` attribute
- `version_format` - Validates that the `version` attribute matches expected format (e.g., `>= 1.0.0`)
- `extra_arguments_valid` - Validates that `extra_arguments` blocks have either a `name` attribute/label and contain `arguments` or nested blocks
- `no_deprecated_fields` - Warns about deprecated block types: `before_hook`, `after_hook` (use plural form), and nested `terraform` (use `source`)

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
  source = "./module"
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
```

### 11. Key-Value Validation (`key_value`)

**Purpose:** Enforce attribute naming conventions, validate values against patterns, and blocklist certain keys.

**Configuration:**

```json
{
  "key_value": {
    "enabled": true,
    "key_case": "snake_case",
    "value_pattern": {
      "region": "^us-[a-z]+-[0-9]+$"
    },
    "disallowed": ["secret", "password"]
  }
}
```

**Checks:**

- `key_case` - Enforce naming convention: `camelCase`, `snake_case`, or `kebab-case`
- `value_pattern` - Regex validation for attribute values (e.g., AWS region format)
- `disallowed` - Blocklist certain attributes that shouldn't exist

**Supported case values:**

- `camelCase` - `^[a-z][a-zA-Z0-9]*$`
- `snake_case` - `^[a-z][a-z0-9_]*$`
- `kebab-case` - `^[a-z][a-z0-9-]*$`

**Example violations:**

```hcl
# key_case violation (snake_case expected)
locals {
  myVar = "test"  # ERROR: should be snake_case
  my-var = "test"  # ERROR: should be snake_case
}

# value_pattern violation
locals {
  region = "invalid"  # WARNING: does not match pattern ^us-[a-z]+-[0-9]+$
}

# disallowed_keys violation
locals {
  secret = "abc"  # ERROR: attribute is not allowed
  password = "123"  # ERROR: attribute is not allowed
}

# Correct usage
locals {
  my_var = "test"  # snake_case ✓
  region = "us-east-1"  # matches pattern ✓
  api_key = "abc"  # allowed key ✓
}
```

### 12. Count/ForEach Validation (`count_for_each`)

**Purpose:** Detect potential issues with count and for_each expressions.

**Configuration:**

```json
{
  "count_for_each": {
    "enabled": true,
    "warn_on_count_zero": true,
    "warn_on_empty_for_each": true,
    "warn_on_conflict": true
  }
}
```

**Checks:**

- `warn_on_count_zero` - Detect `count = 0` which means resource won't be created
- `warn_on_empty_for_each` - Detect empty `for_each = {}` 
- `warn_on_conflict` - Detect when both `count` and `for_each` are used together (they can't be)

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
  count     = 1  # ERROR: cannot use both count and for_each
  for_each = {}
}
```

### 13. Dependency Output Validation (`dependency_outputs`)

**Purpose:** Validate `dependency.*.outputs.*` references by walking the dependency chain.

**Configuration:**

```json
{
  "dependency_outputs": {
    "enabled": true
  }
}
```

**Behavior:**

1. **Dependency resolution**:
   - Parse `dependency` blocks to get `config_path`
   - Support relative paths (e.g., `../vpc`, `vpc`)
   - Skip paths with variables/expressions (shown in verbose mode)
   - Detect circular dependencies to avoid infinite loops

2. **Output validation**:
   - Parse `output` blocks from `.tf` files in dependency module
   - Validate output name exists
   - Error if output doesn't exist

3. **Mock outputs** (for development):
   - Support `.mock-outputs.json` in dependency module directory
   - Format:
     ```json
     {
       "outputs": {
         "vpc_id": { "value": "vpc-123", "type": "string" }
       }
     }
     ```
   - Optional - if not present, only validate against `.tf` files

4. **Error handling**:
   - If output doesn't exist = error
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
  vpc_id = dependency.vpc.outputs.vpc_id  # ✓ validated
  fake   = dependency.vpc.outputs.fake_id  # ✗ not defined in ../vpc/outputs.tf
}
```

## CLI

```bash
# Lint all files
hcl-linter lint ./

# Lint specific files (exact match)
hcl-linter lint ./payments/integration/iam-policy/terragrunt.hcl

# Filter files by name (glob patterns supported, OR logic)
hcl-linter check ./my-dir --filter terragrunt.hcl --filter service.hcl
hcl-linter check ./my-dir --filter "*.hcl"

# Auto-fix issues
hcl-linter fix ./

# Check mode (exit 1 if issues found)
hcl-linter check ./

# Show detailed output
hcl-linter --verbose lint ./

# Use custom config directory
hcl-linter --config-source /path/to/custom-config lint ./

# Set concurrency (number of concurrent workers)
hcl-linter --concurrency 4 lint ./

# Print version information
hcl-linter version
```

**Note:** Files without a matching config (e.g., `something-special.hcl` without `something-special.json`) are warned and skipped.

### Environment Variables

- `HCL_LINTER_CONFIG_DIR`: Path to custom config directory
- `HCL_LINTER_MAX_CONCURRENCY`: Max number of concurrent workers (overrides `--concurrency` flag)

### Concurrency

By default, the linter automatically detects the optimal concurrency level based on CPU count. You can override this:

- CLI flag: `--concurrency <number>`
- Environment variable: `HCL_LINTER_MAX_CONCURRENCY`
- Config: `{"max_concurrency": <number>}` in rules (lowest priority)

Higher concurrency speeds up processing of large file sets but uses more memory.

## Exit Codes

- `0`: All files pass
- `1`: Issues found (when using `--check`)
- `2`: Configuration error

## File Matching

### Config File Matching

Config files are matched by filename:

1. Exact match: `terragrunt.hcl` → `configs/terragrunt.json`
2. Fallback: use `default.json` if exists
3. No config: file is skipped with a warning

### Target File Filtering

When `--filter` is specified:

- Filters are matched as glob patterns against the filename (not full path)
- Multiple `--filter` flags are combined with OR logic
- Matching files are printed before processing
- Non-matching files are silently skipped

Example:

```
$ hcl-linter lint ./infra --filter "*.hcl" --filter "*.tf"
Matched files:
  ./infra/terragrunt.hcl
  ./infra/service/main.tf
  ./infra/shared/vars.hcl
```

## Architecture

```
cmd/hcl-linter/main.go           # CLI entrypoint
internal/
├── config/
│   └── loader.go                # Load configs (multiple sources)
├── linter/
│   ├── linter.go                 # Main linting logic
│   ├── result.go                 # Result types
│   ├── block_order.go            # Block ordering checks
│   ├── array_format.go            # Array format checks
│   ├── validation.go              # Name validation, duplicates, required fields/blocks
│   ├── terragrunt.go               # Terragrunt path and function validation
│   └── terraform.go               # Terraform block validation
├── fix/
│   └── fixer.go                  # Auto-fix logic
└── ast/
    └── parser.go                 # HCL AST utilities
```

## Implementation Notes

- Uses `github.com/hashicorp/hcl/v2` for parsing
- Configuration format: JSON for simplicity and portability

## Build & Release

### Makefile Targets

```bash
# Build binary to dist/
make build

# Build for all platforms (darwin/linux/windows)
make build-all

# Run tests
make test

# Run tests with coverage report
make test-coverage

# Run linter
make lint

# Format and vet code
make fmt
make vet

# Tidy dependencies
make tidy

# Clean build artifacts
make clean

# Install to GOPATH/bin
make install
```

### Versioning

Version is managed via the `VERSION` file:

```
0.1.0
```

Version info is injected at build time via ldflags:

- `main.Version` - from VERSION file
- `main.BuildDate` - build timestamp
- `main.GitCommit` - git SHA

### Release Workflow

```bash
# Tag a release
make tag VERSION=0.1.0

# Build all platforms and generate checksums
make release VERSION=0.1.0
```

This creates:

- `dist/hcl-linter-darwin-amd64`
- `dist/hcl-linter-darwin-arm64`
- `dist/hcl-linter-linux-amd64`
- `dist/hcl-linter-linux-arm64`
- `dist/hcl-linter-windows-amd64.exe`
- `dist/*sha256` checksum files
