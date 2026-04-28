# CLI Reference

## Contents

1. [Commands](#commands)
2. [Global flags](#global-flags)
3. [`lint`](#lint)
4. [`check`](#check)
5. [`fix`](#fix)
   - [`fix --format` flag](#fix---format-flag)
   - [`fix --dry-run` flag](#fix---dry-run-flag)
6. [`validate-config`](#validate-config-command)
7. [`version`](#version)
8. [Target file filtering (`--filter`)](#target-file-filtering---filter)
9. [Concurrency](#concurrency)
10. [Coloured output](#coloured-output)
11. [Environment variables](#environment-variables)
12. [Exit codes](#exit-codes)

## Commands

```bash
# Lint all files under a path
hcl-linter lint ./

# Lint specific files (exact match)
hcl-linter lint ./payments/integration/iam-policy/terragrunt.hcl

# Filter files by name (glob patterns, OR logic)
hcl-linter check ./my-dir --filter terragrunt.hcl --filter service.hcl
hcl-linter check ./my-dir --filter "*.hcl"

# Auto-fix issues (uses configured rules)
hcl-linter fix ./

# Apply default formatting without any config required
hcl-linter fix ./ --format

# Check mode (exit 1 if issues found; same rules as lint)
hcl-linter check ./

# Show detailed output
hcl-linter --verbose lint ./

# Use custom config directory
hcl-linter --config-source /path/to/custom-config lint ./

# Set concurrency (number of concurrent workers)
hcl-linter --concurrency 4 lint ./

# Print version information
hcl-linter version

# Validate config files for typos and misconfigurations
hcl-linter validate-config
hcl-linter validate-config /path/to/.hcl-linter
```

**Note:** Files without a matching config (e.g. `something-special.hcl`
without `.hcl-linter/something-special.hcl` or `.hcl-linter/default.hcl`)
are warned and skipped. Use `fix --format` to apply formatting to any
file without requiring config.

## Global flags

```
--config-source, -c   Config source path (overrides env/defaults)
--filter              Filter files by name pattern (glob, repeatable)
--concurrency         Max concurrent workers (default: CPU count)
--verbose, -v         Show detailed output
--color               Coloured output: auto (default), always, never
```

## `lint`

Lints every matching file under the given path and prints all issues.
Exits `0` regardless of issue count — use `check` if you want a non-zero
exit on issues.

## `check`

Same rule set as `lint`, but exits `1` if any file has issues. Intended
for CI.

## `fix`

Runs the fix pipeline (BlockOrder → NameValidation → RequiredFields →
ArrayFormat → BlankLines) against each matching file and writes the
result back to disk. Rules without a `Fix` implementation are skipped.

Rules are only applied when enabled in the matched config, unless
`--format` is given.

### `fix --format` flag

The `--format` flag applies opinionated default formatting to all HCL
files without requiring any config to be present:

- **Block order:** `include → locals → terraform → dependency → inputs`
- **Array format:** converts inline arrays with 2+ items to multiline
- **Blank lines:** removes blank lines at block boundaries, collapses consecutive blank lines

This is useful for one-off formatting or bootstrapping a codebase before
introducing config-based rules. Config-based `array_format` and
`blank_lines` rules are also applied if a config exists for the file.

### `fix --dry-run` flag

Runs the same Fix pipeline as `fix` but prints a unified diff to stdout per
changed file instead of writing. Exits non-zero (`1`) if any file would be
changed. Composes with `--format`. Intended for CI pre-commit enforcement —
fail the build when committed files aren't byte-identical to what the fixer
would produce.

```bash
hcl-linter fix ./ --dry-run
hcl-linter fix ./ --format --dry-run   # config-free formatting check
```

Differs from `check`: `check` reports rule violations; `--dry-run` reports
byte-level formatting drift, including drift introduced by fix-only rules
like `blank_lines` that have no `Check` phase.

## `validate-config` command

Validates every `.hcl` file in the config directory and exits non-zero if
any issues are found.

```bash
hcl-linter validate-config
hcl-linter validate-config /path/to/.hcl-linter
```

**Checks performed:**

- **Unknown rule blocks** — any block name inside `rules {}` that doesn't
  match a known rule is reported as an error. This catches silent typos:
  the HCL parser ignores unrecognised blocks, so `blokc_order {}` would
  silently do nothing without this check.
- **Enabled rules with missing required fields:**

  | Rule              | Required when enabled                                                         |
  | ----------------- | ----------------------------------------------------------------------------- |
  | `block_order`     | `order` list must be non-empty                                                |
  | `name_validation` | `pattern` must be set                                                         |
  | `required_blocks` | at least one `required` entry must exist                                      |
  | `key_value`       | at least one of `key_case`, `value_pattern`, or `disallowed` must be set      |

**Startup warnings:** `lint`, `check`, and `fix` run the same checks
automatically and print any issues as warnings to stderr. The dedicated
subcommand is useful in CI where you want a hard failure on config
problems.

## `version`

Prints version, build date, and git commit (all injected at build time).

## Target file filtering (`--filter`)

When `--filter` is specified:

- Filters are matched as glob patterns against the filename (not the full path)
- Multiple `--filter` flags are combined with OR logic
- Matching files are printed before processing
- Non-matching files are silently skipped

```
$ hcl-linter lint ./infra --filter "*.hcl" --filter "*.tf"
Matched files:
  ./infra/terragrunt.hcl
  ./infra/service/main.tf
  ./infra/shared/vars.hcl
```

## Concurrency

By default, the linter automatically detects the optimal concurrency
level based on CPU count. You can override this:

- CLI flag: `--concurrency <number>`
- Environment variable: `HCL_LINTER_MAX_CONCURRENCY` (overrides the flag)

Higher concurrency speeds up processing of large file sets but uses more
memory.

## Coloured output

Output is colourised when stdout is an interactive terminal. Control with
the global `--color` flag:

- `--color=auto` (default) — on when stdout is a TTY, `NO_COLOR` is unset, and `TERM` is not `dumb`
- `--color=always` — force colour on (use when piping into a colour-aware pager, e.g. `less -R`)
- `--color=never` — disable colour entirely

Auto mode also honours the `NO_COLOR` env var (https://no-color.org): any
non-empty value disables colour. Explicit `--color=always` overrides
`NO_COLOR`.

### Debugging colour output

**Inspect raw escape codes.** The ANSI escape prefix is `\x1b[` (often
shown as `^[[` or `\033[`). To see what's actually on the wire:

```bash
hcl-linter lint ./ --color=always | od -c | head -20
```

Look for sequences like `\033[31;1m` (red+bold, errors), `\033[33m`
(yellow, warnings), `\033[32m` (green, success), `\033[36;1m` (cyan+bold,
file paths), `\033[36m` (cyan, rule names), `\033[2m` (faint, location
suffixes). Each run is terminated by `\033[0m` or an attribute-specific
unset like `\033[22m`.

Note: on some distros `cat -v` is aliased to `bat`; use `/usr/bin/cat -v`
or `od -c` to bypass the alias.

**Colour missing when expected:**

- `echo $NO_COLOR` — any non-empty value disables colour in auto mode. Unset with `unset NO_COLOR` or pass `--color=always`.
- `echo $TERM` — `dumb` disables colour. Expected values: `xterm-256color`, `screen-256color`, `tmux-256color`, etc.
- `[ -t 1 ] && echo tty || echo not-tty` — confirms whether stdout is a TTY.
- Use `--color=always` to force colour regardless of detection.

**Colour appears garbled in a pager (`^[[31m...`):** the pager isn't
passing ANSI codes through. Use `less -R` or set `PAGER="less -R"`.

**Colour bleeding into redirected files / logs:** use `--color=never`,
or don't pass `--color=always` — auto mode already strips colour for
non-TTY output.

## Environment variables

- `HCL_LINTER_CONFIG_DIR` — Path to custom config directory (priority 2; see [configuration.md](configuration.md#config-source-precedence))
- `HCL_LINTER_MAX_CONCURRENCY` — Max number of concurrent workers (overrides `--concurrency` flag)
- `NO_COLOR` — When set to any non-empty value, disables coloured output in `--color=auto` mode (see https://no-color.org)

## Exit codes

| Code | Meaning                                              |
| ---- | ---------------------------------------------------- |
| `0`  | Success (no issues, or `lint` completed regardless)  |
| `1`  | Issues found (`check`), config validation failed (`validate-config`), or `fix --dry-run` detected files that would be changed |
| `2`  | Configuration error (missing/unparseable config)     |
