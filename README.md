# HCL Linter

<p align="center">
  <img src="docs/assets/hcl-linter-banner.png" alt="HCL Linter" width="680"/>
</p>

<p align="center">
  <a href="https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml"><img src="https://github.com/bard-works/hcl-linter/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/bard-works/hcl-linter/releases/latest"><img src="https://img.shields.io/github/v/release/bard-works/hcl-linter?include_prereleases&sort=semver" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/bard-works/hcl-linter"><img src="https://goreportcard.com/badge/github.com/bard-works/hcl-linter" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/bard-works/hcl-linter" alt="License"></a>
</p>

A linter and formatter for **Terragrunt** and **Terraform** HCL. It catches
the things `terraform validate` can't see and code review keeps bouncing
off: `dependency` paths that don't exist, `find_in_parent_folders()` calls
that find nothing, a `remote_state` with no backend, blocks in the wrong
order, and inconsistent formatting. Most formatting findings fix themselves.

```console
$ hcl-linter check .
live/prod/app/terragrunt.hcl:5:1: [error] block_order: block type "include" appears out of order. Expected: [include locals terraform remote_state generate dependencies dependency]
live/prod/app/terragrunt.hcl:10:17: [error] dependency_path_exists: dependency "vpc": config_path "../network" does not exist
live/prod/app/terragrunt.hcl:14:13: [warning] get_env_has_default: get_env() should have a default value as second argument
live/prod/app/terragrunt.hcl:15:13: [warning] array_format: array with 2 items should list one item per line
root.hcl:1:1: [error] remote_state_backend_required: remote_state block missing required 'backend' attribute
----------------------------------------
Total: 5 issue(s) in 3 file(s)
```

## Try it in 10 seconds

No config needed. See what the formatter would change (nothing is written):

```bash
hcl-linter fix --format --dry-run .
```

`fix --format` orders Terragrunt blocks (`include → locals → terraform →
dependency → inputs`), puts arrays of two or more strings one item per line,
removes stray blank lines inside blocks, then applies the same canonical
formatting as `terraform fmt`. It never reorders list items or touches
heredocs, and a second run changes nothing.

## How it compares

|                                                       | `terraform fmt` / `terragrunt hclfmt` | tflint          | **hcl-linter** |
| ----------------------------------------------------- | ------------------------------------- | --------------- | -------------- |
| Whitespace and alignment                              | ✅                                    |                 | ✅ (same formatter) |
| Block order, array layout, blank lines                |                                       |                 | ✅ auto-fix    |
| Terragrunt: `dependency` / `include` paths exist      |                                       |                 | ✅             |
| Terragrunt: `dependency.x.outputs.y` is declared      |                                       |                 | ✅             |
| Terragrunt: `find_in_parent_folders`, `get_env`, `remote_state` |                             |                 | ✅             |
| Naming, required blocks/attributes, disallowed keys   |                                       | some            | ✅ configurable |
| Provider-aware checks (instance types, deprecated args) |                                     | ✅              |                |

It complements rather than replaces those tools: keep `terraform fmt` and
tflint, and add hcl-linter for Terragrunt wiring and team conventions. The
tools don't fight: `fix --format` output is `terraform fmt`-clean, and `fix`
never makes a `terraform fmt`-clean file fail `terraform fmt -check`.

## Install

```bash
# Homebrew
brew install --cask bard-works/tap/hcl-linter

# Script: latest release to ~/.local/bin, checksum-verified
curl -fsSL https://raw.githubusercontent.com/bard-works/hcl-linter/main/scripts/install.sh | sh

# Go
go install github.com/bard-works/hcl-linter/cmd/hcl-linter@latest

# Docker
docker run --rm -v "$PWD:/src" ghcr.io/bard-works/hcl-linter check .
```

Or download an archive from the [latest release](https://github.com/bard-works/hcl-linter/releases/latest).
Every archive has a build-provenance attestation:
`gh attestation verify <archive> --repo bard-works/hcl-linter`.

## Quick start

```bash
hcl-linter init            # write .hcl-linter/ with the recommended preset for your project
hcl-linter check .         # report issues; exit 1 on errors (use in CI)
hcl-linter fix .           # apply auto-fixes in place
hcl-linter fix . --dry-run # show the diff instead; exit 1 if anything would change
hcl-linter explain         # list rules; `explain <rule>` for details
```

`init` looks at your files and writes a `.hcl-linter/` directory whose
`default.hcl` extends `builtin:terragrunt` (if it finds `terragrunt.hcl` or
`root.hcl`) or `builtin:terraform`. `check .` straight afterwards shows
real findings only.

## Use in CI

**GitHub Actions**, with findings as code-scanning annotations on the PR:

```yaml
jobs:
  hcl-linter:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v6
      - uses: bard-works/hcl-linter@v0.1.0
        with:
          args: check . --output sarif
          output-file: hcl-linter.sarif
      - uses: github/codeql-action/upload-sarif@v4
        if: always()
        with:
          sarif_file: hcl-linter.sarif
```

Or keep it simple with `- uses: bard-works/hcl-linter@v0.1.0` (runs
`check .`), and add `args: fix . --dry-run` to fail on formatting drift.

**pre-commit** (`.pre-commit-config.yaml`):

```yaml
repos:
  - repo: https://github.com/bard-works/hcl-linter
    rev: v0.1.0
    hooks:
      - id: hcl-linter-check    # lint changed .hcl/.tf files
      - id: hcl-linter-fix      # or: hcl-linter-format (no config needed)
```

**GitLab CI, Atlantis, anything with Docker:**

```yaml
hcl-linter:
  image:
    name: ghcr.io/bard-works/hcl-linter:latest
    entrypoint: [""]
  script:
    - hcl-linter check . --output checkstyle > hcl-linter.xml
```

`--output` accepts `text` (default), `json`, `sarif` and `checkstyle`.
Exit codes: `0` clean, `1` findings, `2` usage/config error, `3` a file
couldn't be processed. See [docs/cli.md](docs/cli.md).

## Configuration

Config lives in `.hcl-linter/`, one file per target filename without its
extension: `terragrunt.hcl` uses `.hcl-linter/terragrunt.hcl`, `main.tf`
uses `.hcl-linter/main.hcl`, and anything else falls back to
`.hcl-linter/default.hcl`.

```hcl
# .hcl-linter/default.hcl
extends = "builtin:terragrunt"   # start from the recommended preset

rules {
  # A block here replaces the preset's block of the same name.
  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["dependency", "include"]
  }
}
```

- **Inheritance:** `extends = "default"` pulls in another file from the same
  directory; chains work and cycles are reported.
- **Per-directory overrides:** a `.hcl-linter/` deeper in the tree wins for
  files beneath it, which is handy in monorepos.
- **Inline suppression:** `# hcl-linter:ignore dependency_paths -- reason`
  above a block, or `# hcl-linter:ignore-file array_format` for a whole file.
- **Typos are caught:** `hcl-linter validate-config` fails on unknown rule
  blocks such as `blokc_order` and on broken `extends`.

Details: [docs/configuration.md](docs/configuration.md). Every rule with its
options: [docs/rules.md](docs/rules.md).

## Docs

- [docs/rules.md](docs/rules.md): presets, every rule, config fields, severity
- [docs/configuration.md](docs/configuration.md): config format, precedence, inheritance, suppression
- [docs/cli.md](docs/cli.md): commands, flags, report formats, exit codes
- [docs/architecture.md](docs/architecture.md): package layout, data flow, release process
- [docs/demo/README.md](docs/demo/README.md): a valid and a violating example for every rule

## Contributing

Bug reports, rule ideas and PRs are welcome. [CONTRIBUTING.md](CONTRIBUTING.md)
covers setup, how to add a rule, and the PR workflow.

## Security

Report vulnerabilities privately via [GitHub Security Advisories](https://github.com/bard-works/hcl-linter/security/advisories).
See [SECURITY.md](SECURITY.md).

## License

Apache 2.0
