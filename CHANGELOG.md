# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.1-alpha]

### Added

#### CLI

- `lint` and `check` commands with coloured output and verbose file listing.
- `fix` command with a priority-ordered fix pipeline
  (BlockOrder → NameValidation → RequiredFields → ArrayFormat → BlankLines).
- `fix --format` flag for opinionated default formatting without any config.
- `fix --dry-run` prints a unified diff per file and exits non-zero if any
  file would change. Composes with `--format`. Intended for CI enforcement.
- `validate-config` command detects unknown rule blocks and enabled rules
  with missing required fields. Warnings also surface at startup during
  `lint` / `check` / `fix`.
- `validate-config --recursive` walks a path and validates every nested
  `.hcl-linter/` directory found.
- `explain [rule]` prints rule summary, severity, config fields, and an
  example violation. Without an argument, lists all rules in an aligned
  table.
- `init [path]` scaffolds a `.hcl-linter/` directory. Walks the target for
  `.hcl` / `.tf` files, groups by unique basename, and writes a
  `default.hcl` baseline plus one `extends = "default"` override per unique
  name. Supports `--dry-run` and `--force`.
- `version` command.
- `--color=auto|always|never` global flag. `auto` honours `NO_COLOR` and
  `TERM=dumb`; `always` overrides both. Colours: red for errors, yellow for
  warnings, green for clean summaries.
- `--filter` flag for glob-based file filtering (multi-value, OR logic).
- `--concurrency` flag and `HCL_LINTER_MAX_CONCURRENCY` env var for
  configurable parallel file processing.
- `--include-hidden` flag to include files inside hidden directories.

#### Rules

- `block_order` - enforce top-level and nested block ordering.
- `array_format` - normalize arrays with 2+ items to multiline.
- `blank_lines` - remove unnecessary blank lines within blocks.
- `name_validation` - enforce naming conventions on block labels.
- `required_fields` - enforce required attributes per block type.
- `required_blocks` - enforce block presence with `at_least_one` or `once`
  count semantics.
- `duplicates` - detect duplicate block labels.
- `key_value` - enforce attribute key casing, disallowed keys, and
  value regex patterns.
- `count_for_each` - detect dangling `count = 0` / `for_each = {}` patterns.
- `dependency_paths` - validate `dependency.config_path` exists on disk.
- `include_paths` - validate `include.path` exists on disk.
- `remote_state` - validate `remote_state` blocks have a `backend`.
- `hcl_functions` - warn on `get_env()` without a default; validate
  `find_in_parent_folders()` target exists.
- `terraform_block` - enforce `source`, `version` constraint format, and
  `extra_arguments` structure.
- `dependency_outputs` - statically validate `dependency.*.outputs.*`
  references against `.tf` output blocks and `.mock-outputs.json`.

#### Config

- HCL-only config format. JSON support intentionally excluded.
- Per-filename config matching: `terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`,
  fallback to `.hcl-linter/default.hcl`.
- Per-directory config overrides: a `.hcl-linter/` anywhere in the source
  tree wins for files beneath it. Closer config wins; root config is the
  fallback. Walk is bounded by the parent of the root `.hcl-linter/`.
- `extends` inheritance: configs can inherit from a base config within the
  same `.hcl-linter/` directory.

#### Infrastructure

- `Doc() RuleDoc` interface method on every rule - compiler-enforced.
  Surfaces summary, severity, config fields, and example violations via
  `explain`.
- `ParseError` structured type with file context; `Unwrap()` for proper
  `errors.Is` / `errors.As` support.
- Timeout protection for file-system operations with circuit breaker.
- `SeverityInfo` and `SeverityNotice` diagnostic severity constants.
- `GetBodyAttributes()` helper in `internal/ast` - replaces seven inline
  `body.JustAttributes()` calls.
- Symlink guard and TOCTOU fix in engine and `init` command.
- Cross-platform release builds: darwin amd64/arm64, linux amd64/arm64,
  windows amd64 - with sha256 checksums.
- Homebrew tap formula.
- Weekly Dependabot for Go modules and GitHub Actions.
- GitHub Actions CI with lint, test, coverage, and CodeQL security jobs.

### Changed

- Rule engine unified: all rules self-register via `func init()` into a
  global `DefaultRegistry`; `engine.New()` no longer holds a manual list.
  `Rule` interface gains `Priority() int`; `Fix` now returns `(int, error)`
  - the int is the actual edit count, not a binary changed flag.
- `internal/linter` package renamed to `internal/diag`.
- Binary size reduced ~30% via `-s -w` ldflags.
- `isLowerLetter` renamed to `isValidStartChar` / `isValidIdentifierChar`.
- `defaultNamePattern` extracted as a named constant in `name_validation`.

### Removed

- JSON config file support.
- `internal/fix` package (absorbed into the rule engine).
