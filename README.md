# HCL Linter

<p align="center">
  <img src="docs/assets/hcl-linter-banner.png" alt="HCL Linter" width="680"/>
</p>

A configurable linter for Terragrunt HCL files that enforces consistency standards across large codebases.

[![CI](https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml/badge.svg)](https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bard-works/hcl-linter)](https://github.com/bard-works/hcl-linter)

## Features

- **Block ordering** - Enforce consistent ordering of top-level and nested blocks
- **Array formatting** - Normalize array formatting (inline vs multiline)
- **Name validation** - Enforce naming conventions for block labels
- **Duplicate detection** - Detect duplicate block definitions
- **Required fields** - Enforce required attributes per block type
- **Required blocks** - Enforce presence of required block types
- **Terragrunt validation** - Validate paths, include files, and remote state config
- **Terragrunt functions** - Validate `find_in_parent_folders()` and `get_env()` calls
- **Terraform block** - Validate terraform blocks for source, version, and deprecated fields
- Auto-fix capability for formatable issues
- Configurable rules per filename pattern
- Fast processing with configurable concurrency

## Install

### Homebrew

```bash
brew install bard-works/tap/hcl-linter
```

### Binary Download

Download pre-built binaries from the [latest release](https://github.com/bard-works/hcl-linter/releases/latest).

### Build from Source

```bash
go install github.com/bard-works/hcl-linter@latest
```

## Quick Start

```bash
# Lint all files
hcl-linter lint ./

# Check and exit 1 if issues found
hcl-linter check ./

# Auto-fix formatting issues (uses configured rules)
hcl-linter fix ./

# Apply default formatting without any config required
hcl-linter fix ./ --format
```

## Configuration

Create a `.hcl-linter/` directory with config files:

```
.hcl-linter/
├── default.hcl      # Base config for all files
├── terragrunt.hcl   # Rules for terragrunt.hcl
└── root.hcl         # Rules for root.hcl
```

Example `default.hcl`:

```hcl
rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "dependency", "inputs"]
  }

  required_blocks {
    required {
      type  = "terraform"
      count = "once"
      error = "missing terraform block"
    }
  }
}
```

## Formatting

The `fix` command auto-fixes formatting issues:

- **Blank lines** - Removes extra blank lines within blocks, trims excess newlines
- **Array format** - Converts inline arrays to multiline when threshold exceeded
- **Block order** - Reorders blocks to match configured order

Use `--format` to apply default formatting to any file without needing a config:

```bash
hcl-linter fix ./ --format
```

This applies: block ordering (`include → locals → terraform → dependency → inputs`), array normalization, and blank line cleanup.

Enable rules in your config:

```hcl
rules {
  blank_lines {
    enabled        = true
    within_blocks = true
  }

  array_format {
    enabled            = true
    multiline_threshold = 2
  }

  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}
```

Example - fix extra blank lines:

```hcl
# Before fix
inputs = {

  name = "test"


  options = "none"

}

# After fix
inputs = {
  name = "test"

  options = "none"
}
```

```hcl
rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}
```

## CLI Options

```bash
--config-source, -c  Config source path
--filter             Filter files by name pattern (glob supported)
--concurrency        Max concurrent workers (default: CPU count)
--verbose, -v        Show detailed output

# fix-only flag:
--format             Apply default formatting without requiring config rules
```

## Environment Variables

- `HCL_LINTER_CONFIG_DIR` - Path to config directory
- `HCL_LINTER_MAX_CONCURRENCY` - Max concurrent workers

## Full Documentation

See [SPEC.md](SPEC.md) for complete documentation including all rules and configuration options.

## License

Apache 2.0
