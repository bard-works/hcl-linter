# HCL Linter - Specification

A configurable linter for Terragrunt HCL files that enforces consistency standards across large codebases.

## Goals

- Enforce block ordering, formatting, naming, required fields, blank lines, and required blocks
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

  repository = "test"


  tags = "value"

}

# After
inputs = {
  repository = "test"

  tags = "value"
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
include "vault-azuread" {}

# Correct
include "vault_azuread" {}
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
