# HCL Linter

<p align="center">
  <img src="docs/assets/hcl-linter-banner.png" alt="HCL Linter" width="680"/>
</p>

A configurable linter and auto-fixer for HCL 2. Ships with rule sets for Terragrunt and Terraform, and works against any HCL file.

[![CI](https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml/badge.svg)](https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bard-works/hcl-linter)](https://github.com/bard-works/hcl-linter)

Written for teams with more than one person touching HCL. Enforces block order, naming, required fields, path references, and formatting — the things code review keeps bouncing off. Rules are per-file-pattern and inherit via `extends`. Fixable rules auto-fix; CI uses `check` or `fix --dry-run`.

## Install

```bash
# Homebrew
brew install bard-works/tap/hcl-linter

# go install
go install github.com/bard-works/hcl-linter/cmd/hcl-linter@latest
```

Pre-built binaries: [latest release](https://github.com/bard-works/hcl-linter/releases/latest).

## Quick start

```bash
hcl-linter init                # bootstrap .hcl-linter/ for the current project
hcl-linter lint ./             # lint (exit 0 regardless)
hcl-linter check ./            # lint, exit 1 on any issue — use in CI
hcl-linter fix ./              # apply fixable rules in-place
hcl-linter fix ./ --format     # apply default formatting, no config needed
hcl-linter fix ./ --dry-run    # print diff, exit 1 if anything would change
hcl-linter explain             # list rules; `explain <name>` for details
```

## Configuration

Config lives in `.hcl-linter/`, one file per target filename. `terragrunt.hcl` is matched by `.hcl-linter/terragrunt.hcl`, falling back to `.hcl-linter/default.hcl`.

```
.hcl-linter/
├── default.hcl      # fallback for any file with no specific match
├── terragrunt.hcl   # rules for terragrunt.hcl
└── root.hcl         # rules for root.hcl
```

```hcl
# .hcl-linter/default.hcl
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

### Inheritance

`extends = "default"` pulls in another config from the same directory. Child rule blocks override the base rule block wholesale; unset rules are inherited. Chains work; circular refs error.

```hcl
# .hcl-linter/terragrunt.hcl
extends = "default"

rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}
```

### Per-directory overrides

A `.hcl-linter/` directory anywhere in the tree overrides rules for files beneath it. Closer wins; no cross-directory merging. Useful in monorepos. See [docs/configuration.md](docs/configuration.md#per-directory-overrides).

### `fix --format`

Applies opinionated defaults without a config file: block order `include → locals → terraform → dependency → inputs`, multiline arrays at 2+ items, blank-line cleanup. Good for one-off formatting or bootstrapping before config-based rules.

## CLI flags

```
--config-source, -c   Config directory (overrides env/defaults)
--filter              Glob filter on filename, repeatable (OR)
--concurrency         Max concurrent workers (default: CPU count)
--verbose, -v         Detailed output
--color               auto (default) | always | never

# fix only
--format              Apply defaults without config
--dry-run             Print diff, exit 1 on any change
```

## Environment

- `HCL_LINTER_CONFIG_DIR` — config directory
- `HCL_LINTER_MAX_CONCURRENCY` — worker count
- `NO_COLOR` — disables colour in `--color=auto` (https://no-color.org)

## Validating configs

Typos in rule block names (`blokc_order`) parse cleanly as unknown HCL and silently do nothing. `lint`, `check`, and `fix` print warnings for these at startup. For a hard failure in CI:

```bash
hcl-linter validate-config
hcl-linter validate-config . --recursive
```

## Docs

- [docs/rules.md](docs/rules.md) — every rule, config fields, severity
- [docs/configuration.md](docs/configuration.md) — config format, precedence, inheritance
- [docs/cli.md](docs/cli.md) — commands, flags, exit codes, colour
- [docs/architecture.md](docs/architecture.md) — package layout and data flow

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) covers setup, rule conventions, and PR workflow.

## Security

Report vulnerabilities via [GitHub Security Advisories](https://github.com/bard-works/hcl-linter/security/advisories). See [SECURITY.md](SECURITY.md).

## License

Apache 2.0
