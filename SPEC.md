# HCL Linter - Specification

A configurable HCL linter that enforces consistency standards across large
codebases. The core rules (block ordering, formatting, naming, required fields,
blank lines, required blocks, key-value validation, etc.) work against any
HCL 2 file. Additional rule sets ship for Terragrunt and Terraform.

## Goals

- Enforce block ordering, formatting, naming, required fields, blank lines, required blocks, and key-value validations for any HCL 2 file
- Ship built-in rule sets for Terragrunt and Terraform concerns (dependency paths, include paths, remote state, function usage, etc.)
- Configurable rules per filename pattern (e.g. `terragrunt.hcl`, `root.hcl`, `service.hcl`, or any filename your project uses)
- Auto-fix capability for formatable issues
- Extensible config system with user-defined configurations
- Fast processing with configurable concurrency

## Configuration

Configuration files in `.hcl-linter/` directory, named after the file pattern they configure:

```
.hcl-linter/
├── default.hcl       # Base config for all files (optional)
├── terragrunt.hcl    # Rules for terragrunt.hcl
├── root.hcl          # Rules for root.hcl
└── service.hcl       # Rules for service.hcl
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

**Full override**: User config completely replaces project configs - no merging across source locations. Within a single config directory, use `extends` for inheritance (see below).

**Applied configs are printed at startup** for transparency.

If falling back to project defaults (no user config found), a warning is displayed.

### Config Inheritance (`extends`)

A config file can inherit from another config in the same directory using the top-level `extends` attribute:

```hcl
extends = "default"

rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}
```

**Behavior:**

- `extends` takes a config name (with or without `.hcl` extension) relative to the same directory
- Base config is loaded first; child rules override the base at the rule-block level (whole rule blocks, not individual fields)
- Unset rules in the child are inherited from the base unchanged
- Chains are supported: `child extends parent extends grandparent`
- Circular references are detected and reported as an error

**Example — shared base with per-file overrides:**

```
.hcl-linter/
├── default.hcl       # Shared base rules (block_order, blank_lines, etc.)
├── terragrunt.hcl    # extends = "default", overrides block_order
└── root.hcl          # extends = "default", adds required_blocks
```

### Configuration Schema (HCL)

```hcl
rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "dependency", "inputs"]
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
    blocks  = ["include", "dependency"]
  }

  duplicates {
    enabled = true
    blocks  = ["locals", "dependency", "include"]
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
    enabled   = true
    key_case  = "snake_case"
    value_pattern = {
      region = "^us-[a-z]+-[0-9]+$"
    }
    disallowed = ["secret", "password"]
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
```

## Rules

### 1. Block Order (`block_order`)

**Purpose:** Enforce consistent ordering of top-level blocks.

**Behavior:**

- Order of listed blocks is enforced
- Unlisted blocks (e.g., `outputs`, custom blocks) are placed after the last listed block

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

**Configuration:**

```hcl
rules {
  dependency_paths {
    enabled = true
  }
}
```

**Rule ID emitted:** `dependency_path_exists`

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

**Configuration:**

```hcl
rules {
  include_paths {
    enabled = true
  }
}
```

**Rule ID emitted:** `include_path_exists`

**Example violation:**

```hcl
include "root" {
  path = "non-existent/parent.hcl"  # ERROR: file does not exist
}
```

### 10. Remote State (`remote_state`)

**Purpose:** Validate `remote_state { }` blocks nested inside the top-level
`terraform { }` block. When `require_backend = true`, the `backend` attribute
must be set and non-empty.

**Configuration:**

```hcl
rules {
  remote_state {
    enabled         = true
    require_backend = true
  }
}
```

**Rule ID emitted:** `remote_state_backend_required`

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

### 12. Terraform Block (`terraform_block`)

**Purpose:** Validate Terragrunt's `terraform` block configuration.

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

### 13. Key-Value Validation (`key_value`)

**Purpose:** Enforce attribute naming conventions, validate values against patterns, and blocklist certain keys.

**Configuration:**

```hcl
rules {
  key_value {
    enabled      = true
    key_case     = "snake_case"
    value_pattern = {
      region = "^us-[a-z]+-[0-9]+$"
    }
    disallowed = ["secret", "password"]
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

### 14. Count/ForEach Validation (`count_for_each`)

**Purpose:** Detect potential issues with count and for_each expressions.

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

### 15. Dependency Output Validation (`dependency_outputs`)

**Purpose:** Validate `dependency.*.outputs.*` references by walking the dependency chain.

**Configuration:**

```hcl
rules {
  dependency_outputs {
    enabled = true
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
   - Format: `{"outputs": {"vpc_id": {"value": "vpc-123", "type": "string"}}}`
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

# Auto-fix issues (uses configured rules)
hcl-linter fix ./

# Apply default formatting without any config required
hcl-linter fix ./ --format

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

# Validate config files for typos and misconfigurations (exits non-zero on errors)
hcl-linter validate-config
hcl-linter validate-config /path/to/.hcl-linter
```

**Note:** Files without a matching config (e.g., `something-special.hcl` without a `something-special.hcl` or `default.hcl` in `.hcl-linter/`) are warned and skipped. Use `--format` to apply formatting to any file without requiring config.

### `validate-config` Command

Validates every `.hcl` file in the config directory and exits non-zero if any issues are found.

**Checks performed:**

- **Unknown rule blocks** — any block name inside `rules {}` that doesn't match a known rule is reported as an error. This catches silent typos: the HCL parser ignores unrecognised blocks, so `blokc_order {}` would silently do nothing without this check.
- **Enabled rules with missing required fields:**

| Rule | Required when enabled |
|------|-----------------------|
| `block_order` | `order` list must be non-empty |
| `name_validation` | `pattern` must be set |
| `required_blocks` | at least one `required` entry must exist |
| `key_value` | at least one of `key_case`, `value_pattern`, or `disallowed` must be set |

**Startup warnings:** `lint`, `check`, and `fix` run the same checks automatically and print any issues as warnings to stderr. The dedicated subcommand is useful in CI where you want a hard failure on config problems.

### `fix --format` Flag

The `--format` flag applies opinionated default formatting to all HCL files without requiring any config to be present:

- **Block order**: `include → locals → terraform → dependency → inputs`
- **Array format**: converts inline arrays with 2+ items to multiline
- **Blank lines**: removes blank lines at block boundaries, collapses consecutive blank lines

This is useful for one-off formatting or bootstrapping a codebase before introducing config-based rules. Config-based `array_format` and `blank_lines` rules are also applied if a config exists for the file.

### Environment Variables

- `HCL_LINTER_CONFIG_DIR`: Path to custom config directory
- `HCL_LINTER_MAX_CONCURRENCY`: Max number of concurrent workers (overrides `--concurrency` flag)

### Concurrency

By default, the linter automatically detects the optimal concurrency level based on CPU count. You can override this:

- CLI flag: `--concurrency <number>`
- Environment variable: `HCL_LINTER_MAX_CONCURRENCY`

Higher concurrency speeds up processing of large file sets but uses more memory.

### Coloured Output

Output is colourised when stdout is an interactive terminal. Control with the
global `--color` flag:

- `--color=auto` (default) — on when stdout is a TTY, `NO_COLOR` is unset, and `TERM` is not `dumb`
- `--color=always` — force colour on (use when piping into a colour-aware pager, e.g. `less -R`)
- `--color=never` — disable colour entirely

Auto mode also honours the `NO_COLOR` env var (https://no-color.org): any
non-empty value disables colour. Explicit `--color=always` overrides `NO_COLOR`.

#### Debugging colour output

**Inspect raw escape codes.** The ANSI escape prefix is `\x1b[` (often shown as
`^[[` or `\033[`). To see what's actually on the wire:

```bash
hcl-linter lint ./ --color=always | od -c | head -20
```

Look for sequences like `\033[31;1m` (red+bold, errors), `\033[33m` (yellow,
warnings), `\033[32m` (green, success), `\033[36;1m` (cyan+bold, file paths),
`\033[36m` (cyan, rule names), `\033[2m` (faint, location suffixes). Each run
is terminated by `\033[0m` or an attribute-specific unset like `\033[22m`.

Note: on some distros `cat -v` is aliased to `bat`; use `/usr/bin/cat -v` or
`od -c` to bypass the alias.

**Colour missing when expected:**

- `echo $NO_COLOR` — any non-empty value disables colour in auto mode. Unset it with `unset NO_COLOR` or pass `--color=always`.
- `echo $TERM` — `dumb` disables colour. Expected values: `xterm-256color`, `screen-256color`, `tmux-256color`, etc.
- `[ -t 1 ] && echo tty || echo not-tty` — confirms whether stdout is a TTY.
- Use `--color=always` to force colour regardless of detection.

**Colour appears garbled in a pager (`^[[31m...`):** the pager isn't passing
ANSI codes through. Use `less -R` or set `PAGER="less -R"`.

**Colour bleeding into redirected files / logs:** use `--color=never`, or
don't pass `--color=always` — auto mode already strips colour for non-TTY
output.

**Implementation:** colour handling lives in `internal/termcolor/`. Semantic
helpers (`Error`, `Warning`, `Success`, `Path`, `Rule`, `Location`) wrap
strings via `github.com/fatih/color`. `SetMode` is called once per invocation
from cobra's `PersistentPreRunE`. One subtlety: `fatih/color.New()` bakes a
per-instance `noColor` flag at construction time if `NO_COLOR` was set in env,
so `SetMode` must resync each cached `*Color` via `EnableColor()` /
`DisableColor()` — toggling the package-level `color.NoColor` alone is not
sufficient. This is regression-tested in `termcolor_test.go`.

## Exit Codes

- `0`: All files pass
- `1`: Issues found (when using `--check`)
- `2`: Configuration error

## File Matching

### Config File Matching

Config files are matched by filename:

1. Exact match: `terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`
2. Fallback: use `.hcl-linter/default.hcl` if exists
3. No config: file is skipped with a warning

A matched config may use `extends` to inherit from another config in the same directory (see [Config Inheritance](#config-inheritance-extends) above).

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

### Package layout

```
cmd/hcl-linter/main.go       # CLI entrypoint (cobra: lint, check, fix, version)
internal/
├── engine/
│   └── engine.go            # Engine: single entry point for all lint and fix ops
│                            #   LintFile / LintFiles / FixFile / FixFiles
│                            #   FormatFixFile / FormatFixFiles
├── rules/
│   ├── rule.go              # Rule, Fixer interfaces; Context struct
│   ├── registry.go          # Registry: Register / Enabled / All
│   ├── block_order.go       # BlockOrderRule      — Check + Fix
│   ├── array_format.go      # ArrayFormatRule     — Check + Fix
│   ├── blank_lines.go       # BlankLinesRule      — Fix only
│   ├── name_validation.go   # NameValidationRule  — Check + Fix
│   ├── required_fields.go   # RequiredFieldsRule  — Check + Fix
│   ├── required_blocks.go   # RequiredBlocksRule  — Check
│   ├── duplicates.go        # DuplicatesRule      — Check
│   ├── terragrunt.go        # TerragruntRule      — Check
│   ├── terragrunt_functions.go  # TerragruntFunctionsRule — Check
│   ├── terraform_block.go   # TerraformBlockRule  — Check
│   ├── key_value.go         # KeyValueRule        — Check
│   ├── count_foreach.go     # CountForEachRule    — Check
│   └── dependency_outputs.go # DependencyOutputsRule — Check
├── config/
│   ├── loader.go            # Config discovery and loading
│   ├── hcl.go               # HCL config parser + extends resolution
│   └── types.go             # Config structs (Rules, BlockOrderConfig, …)
├── linter/
│   └── result.go            # Types only: Issue, Result, Severity
└── ast/
    └── parser.go            # HCL parse helpers (blocks, attributes, expressions)
```

### Data flow

```
                    ┌─────────────────────────────────┐
                    │           CLI (cobra)            │
                    │    lint / check / fix / version  │
                    └────────────────┬────────────────┘
                                     │ path(s)
                    ┌────────────────▼────────────────┐
                    │             Engine              │
                    │  buildContext(path)              │
                    │    ├─ Config Loader              │
                    │    │    LoadForFile → *Rules     │
                    │    ├─ AST Parser                 │
                    │    │    ParseFile → *hcl.File    │
                    │    │    GetTopLevelBlocks/Attrs  │
                    │    └─ → Context{FilePath,        │
                    │           Content, File,         │
                    │           Blocks, Attrs, Config} │
                    └────────────────┬────────────────┘
                                     │ ctx
                    ┌────────────────▼────────────────┐
                    │    Registry.Enabled(cfg)        │
                    │    returns rules where           │
                    │    rule.Enabled(cfg) == true     │
                    └──┬───────┬──────┬──────┬────────┘
                       │       │      │      │
              ┌────────▼──┐ ┌──▼──┐ ┌─▼──┐ ┌▼──────┐
              │BlockOrder │ │Name │ │ … │ │Dep    │
              │Check / Fix│ │Valid│ │   │ │Outputs│
              └───────────┘ └─────┘ └───┘ └───────┘
                       │       │      │      │
                    ┌──▼───────▼──────▼──────▼──────┐
                    │   []Issue (lint) or []byte (fix)│
                    └────────────────────────────────┘
```

### Rule and Fixer interfaces

```go
// Every rule implements Rule.
type Rule interface {
    Name()    string
    Enabled(cfg *config.Rules) bool
    Check(ctx *Context) []diag.Issue
}

// Rules that can auto-correct also implement Fixer.
type Fixer interface {
    Rule
    Fix(ctx *Context) ([]byte, bool, error)
}
```

Fix is applied by the engine in a fixed pipeline order (BlockOrder → NameValidation →
RequiredFields → ArrayFormat → BlankLines) rather than via the registry, because each
fix step re-parses the file before the next one runs.

## Implementation Notes

- Uses `github.com/hashicorp/hcl/v2` for parsing
- Configuration format: HCL (same format as the files being linted)

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
