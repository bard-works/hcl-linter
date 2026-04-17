# HCL Linter - Specification

A configurable linter for Terragrunt HCL files that enforces consistency standards across large codebases.

## Goals

- Enforce block ordering, formatting, naming, and required fields
- Configurable rules per filename pattern (terragrunt.hcl, root.hcl, service.hcl)
- Auto-fix capability for formatable issues
- Extensible config system with user-defined configurations

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

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | CLI `--config-source` | Explicit path via flag |
| 2 | Env `HCL_LINTER_CONFIG_DIR` | Environment variable |
| 3 | `.hcl-linter/` in cwd | User config in current directory |
| 4 | `.hcl-linter/` in home | User config in home directory |
| 5 | Project's `.hcl-linter/` | Built-in defaults (fallback) |

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

### 3. Name Validation (`name_validation`)

**Purpose:** Ensure consistent naming conventions.

**Checks:**
- Include/dependency block labels must match regex pattern
- Default: `^[a-z][a-z0-9_]*$` (no hyphens)

**Example violation:**
```hcl
# Wrong
include "vault-azuread" {}

# Correct
include "vault_azuread" {}
```

### 4. Duplicate Detection (`duplicates`)

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

### 5. Required Fields (`required_fields`)

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
```

**Note:** Files without a matching config (e.g., `something-special.hcl` without `something-special.json`) are warned and skipped.

### Environment Variables

- `HCL_LINTER_CONFIG_DIR`: Path to custom config directory

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
│   ├── linter.go                 # Linting logic
│   └── result.go                 # Result types
├── fix/
│   └── fixer.go                  # Auto-fix logic
└── ast/
    └── parser.go                 # HCL AST utilities
```

## Implementation Notes

- Uses `github.com/hashicorp/hcl/v2` for parsing
- Configuration format: JSON for simplicity and portability
