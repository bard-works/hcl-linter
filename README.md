# HCL Linter

<p align="center">
  <img src="docs/assets/hcl-linter-banner.png" alt="HCL Linter" width="680"/>
</p>

A configurable HCL linter that enforces consistency standards across large codebases. Ships with built-in rule sets for Terragrunt and Terraform, and works against any HCL 2 file.

[![CI](https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml/badge.svg)](https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bard-works/hcl-linter)](https://github.com/bard-works/hcl-linter)

## Features

- **Block ordering** - Enforce consistent ordering of top-level and nested blocks
- **Array formatting** - Normalize array formatting (inline vs multiline)
- **Blank lines** - Remove excess blank lines within blocks
- **Name validation** - Enforce naming conventions for block labels
- **Duplicate detection** - Detect duplicate block definitions
- **Required fields** - Enforce required attributes per block type
- **Required blocks** - Enforce presence of required block types
- **Dependency paths** - Validate `config_path` on `dependency` blocks points to an existing directory
- **Include paths** - Validate `path` on `include` blocks points to an existing file or directory
- **Remote state** - Require `backend` on `remote_state` blocks nested inside the `terraform` block
- **HCL functions** - Validate common HCL function calls (`find_in_parent_folders()`, `get_env()`)
- **Terraform block** - Validate the `terraform { }` HCL block for source, version, and deprecated fields
- **Key-value validation** - Enforce key case, value patterns, and disallowed attributes
- **Count/for_each** - Detect count=0, empty for_each, and count+for_each conflicts
- **Dependency outputs** - Validate `dependency.*.outputs.*` references against `.tf` files
- Auto-fix capability for formatable issues
- Config inheritance via `extends`
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

### Config Inheritance

Use `extends` to inherit from another config in the same directory. Child rules override the base rule block entirely; unset rules are inherited as-is.

```hcl
# .hcl-linter/terragrunt.hcl — inherits all rules from default, overrides block_order
extends = "default"

rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}
```

Chains are supported (`A extends B extends C`). Circular references are detected and reported as errors.

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
    enabled       = true
    within_blocks = true
  }

  array_format {
    enabled             = true
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

## Validating Config Files

Config files are plain HCL — typos in rule block names (e.g. `blokc_order`) are silently ignored by the parser. Use `validate-config` to catch these before they cause silent no-ops:

```bash
hcl-linter validate-config
hcl-linter validate-config /path/to/.hcl-linter
```

Exits non-zero and prints errors if any config file contains:

- **Unknown rule blocks** — block names that don't match any known rule (likely a typo)
- **Enabled rules with missing required fields** — e.g. `block_order` with no `order` list, `name_validation` with no `pattern`

During normal `lint`, `check`, and `fix` runs the same checks run automatically and print warnings to stderr — no separate step needed in CI unless you want a hard failure.

## CLI Options

```bash
--config-source, -c  Config source path
--filter             Filter files by name pattern (glob supported)
--concurrency        Max concurrent workers (default: CPU count)
--verbose, -v        Show detailed output
--color              Colour output: auto (default), always, never

# fix-only flag:
--format             Apply default formatting without requiring config rules
```

In `--color=auto` (the default), colour is enabled only when stdout is a TTY
and neither `NO_COLOR` nor `TERM=dumb` is set. Use `--color=always` to force
colour through pipes (e.g. `less -R`) or `--color=never` to disable it.

## Environment Variables

- `HCL_LINTER_CONFIG_DIR` - Path to config directory
- `HCL_LINTER_MAX_CONCURRENCY` - Max concurrent workers
- `NO_COLOR` - When set to any non-empty value, disables coloured output in `--color=auto` mode (see https://no-color.org)

## Full Documentation

See [SPEC.md](SPEC.md) for complete documentation including all rules and configuration options.

## License

Apache 2.0
