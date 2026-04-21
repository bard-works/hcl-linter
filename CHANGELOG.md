# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-04-21

Initial public release.

### Added

- Rule engine with `Rule` / `Fixer` interfaces and a fixed fix pipeline
  (BlockOrder → NameValidation → RequiredFields → ArrayFormat → BlankLines).
- Core HCL rules: `block_order` (with nested block ordering), `array_format`,
  `blank_lines`, `name_validation`, `duplicates`, `required_fields`,
  `required_blocks`, `key_value`, `count_for_each`.
- Terragrunt-specific rules: `dependency_paths`, `include_paths`,
  `remote_state`, `hcl_functions` (find_in_parent_folders, get_env),
  `terraform_block`, `dependency_outputs` (static validation against
  `.tf` output blocks and `.mock-outputs.json`).
- CLI commands: `lint`, `check`, `fix`, `validate-config`, `version`.
- `fix --format` flag for opinionated default formatting without any config.
- Config discovery with precedence (CLI flag → env var → cwd → home →
  project defaults) and `extends` inheritance within a config directory.
- Per-filename config matching (e.g. `.hcl-linter/terragrunt.hcl`,
  `.hcl-linter/root.hcl`, fallback to `.hcl-linter/default.hcl`).
- `validate-config` command and startup warnings that catch unknown rule
  blocks and enabled rules with missing required fields.
- Coloured terminal output with `--color=auto|always|never`, `NO_COLOR`
  and `TERM=dumb` detection.
- File filtering via `--filter` (glob, multi-value, OR logic).
- Configurable concurrency via `--concurrency` and
  `HCL_LINTER_MAX_CONCURRENCY`.
- Cross-platform release builds (darwin amd64/arm64, linux amd64/arm64,
  windows amd64) with sha256 checksums.
- Homebrew tap formula.

[Unreleased]: https://github.com/bard-works/hcl-linter/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bard-works/hcl-linter/releases/tag/v0.1.0
