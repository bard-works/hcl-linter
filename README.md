# HCL Linter

<p align="center">
  <img src="hcl-linter-banner.png" alt="HCL Linter" width="680"/>
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

# Auto-fix issues
hcl-linter fix ./
```

## Configuration

Create a `.hcl-linter/` directory with config files:

```
.hcl-linter/
├── default.json       # Base config for all files
├── terragrunt.json   # Rules for terragrunt.hcl
└── root.json         # Rules for root.hcl
```

Example `default.json`:

```json
{
  "rules": {
    "block_order": {
      "enabled": true,
      "order": ["include", "locals", "terraform", "dependency", "inputs"]
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

## CLI Options

```bash
--config-source, -c  Config source path
--filter             Filter files by name pattern (glob supported)
--concurrency        Max concurrent workers (default: CPU count)
--verbose, -v        Show detailed output
```

## Environment Variables

- `HCL_LINTER_CONFIG_DIR` - Path to config directory
- `HCL_LINTER_MAX_CONCURRENCY` - Max concurrent workers

## Full Documentation

See [SPEC.md](SPEC.md) for complete documentation including all rules and configuration options.

## License

Apache 2.0
