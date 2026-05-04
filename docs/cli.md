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
7. [`init`](#init)
8. [`explain`](#explain)
9. [`version`](#version)
10. [Target file filtering (`--filter`)](#target-file-filtering---filter)
11. [Concurrency](#concurrency)
12. [Coloured output](#coloured-output)
13. [Environment variables](#environment-variables)
14. [Exit codes](#exit-codes)

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

# Bootstrap a .hcl-linter/ directory for the project
hcl-linter init
hcl-linter init ./my-project --dry-run

# List all rules with severity and summary
hcl-linter explain

# Show full documentation for one rule
hcl-linter explain block_order
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
Exits `0` regardless of issue count - use `check` if you want a non-zero
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

Runs the fix pipeline but prints a unified diff per changed file instead
of writing. Exits `1` if any file would change. Composes with `--format`.

```bash
hcl-linter fix ./ --dry-run
hcl-linter fix ./ --format --dry-run   # config-free formatting check
```

Not the same as `check`: `check` reports rule violations, `--dry-run`
reports byte-level drift - including drift from fix-only rules like
`blank_lines` that have no `Check` phase. Use `--dry-run` in CI to fail
the build when committed files don't match the fixer's output.

## `validate-config` command

Validates every `.hcl` file in the config directory and exits non-zero if
any issues are found.

```bash
hcl-linter validate-config
hcl-linter validate-config /path/to/.hcl-linter
hcl-linter validate-config . --recursive   # validate every nested .hcl-linter/
```

**`--recursive` flag:** walks the target path and validates every
`.hcl-linter/` directory found (skipping hidden siblings). Exits non-zero if
any directory contains config issues. Useful when per-directory overrides
are in play - see
[configuration.md → Per-directory overrides](configuration.md#per-directory-overrides).

**Checks performed:**

- **Unknown rule blocks** - any block name inside `rules {}` that doesn't
  match a known rule is reported as an error. This catches silent typos:
  the HCL parser ignores unrecognised blocks, so `blokc_order {}` would
  silently do nothing without this check.
- **Enabled rules with missing required fields:**

  | Rule              | Required when enabled                                                    |
  | ----------------- | ------------------------------------------------------------------------ |
  | `block_order`     | `order` list must be non-empty                                           |
  | `name_validation` | `pattern` must be set                                                    |
  | `required_blocks` | at least one `required` entry must exist                                 |
  | `key_value`       | at least one of `key_case`, `value_pattern`, or `disallowed` must be set |

**Startup warnings:** `lint`, `check`, and `fix` run the same checks
automatically and print any issues as warnings to stderr. The dedicated
subcommand is useful in CI where you want a hard failure on config
problems.

## `init`

Bootstraps a `.hcl-linter/` config directory for the current project.

```bash
hcl-linter init                    # uses cwd
hcl-linter init ./my-project
hcl-linter init . --dry-run        # preview without writing
hcl-linter init . --force          # overwrite existing .hcl-linter/
```

Walks the target directory for `.hcl` / `.tf` files (skipping hidden dirs),
groups them by unique basename, then writes:

- `.hcl-linter/default.hcl` - baseline template with `block_order`,
  `array_format`, and `blank_lines` enabled with safe defaults.
- `.hcl-linter/<name>.hcl` per unique basename - a thin
  `extends = "default"` override with an empty `rules {}` block, ready for
  per-filename customisation.

**Flags:**

- `--dry-run` - print the proposed layout and file contents to stdout; no
  files are written.
- `--force` - overwrite existing `.hcl-linter/` contents. Without this flag,
  `init` refuses when the target `.hcl-linter/` directory is non-empty and
  lists what it found.

The generated layout is designed to pass `validate-config` unchanged - run
`hcl-linter validate-config` and `hcl-linter fix ./ --dry-run` as suggested
next steps.

## `explain`

Prints documentation for lint rules.

```bash
hcl-linter explain               # table of all rules: name, severity, fixable, summary
hcl-linter explain block_order   # full detail: config fields, example violation/fix
hcl-linter explain bogus         # exits 1, prints "unknown rule ..."
```

**No-arg output** - one row per rule, aligned with `tabwriter`:

```
RULE                   SEVERITY  FIXABLE  SUMMARY
array_format           warning   yes      Normalizes inline arrays with 2+ items to multiline
block_order            error     yes      Ensures top-level blocks appear in configured order
...
```

**Single-rule output** - summary, all config fields, and example snippet:

```
block_order  [error, fixable]

  Ensures top-level blocks appear in the configured order.

  Config (rules { block_order { ... } }):
    enabled        bool       required  Activate the rule
    order          []string   required  Block types in desired sequence
    ...

  Example violation:
    terraform {}
    include "root" {}

  After fix:
    include "root" {}
    terraform {}
```

Rule name matching is exact - no fuzzy search.

## `version`

Prints version, build date, and git commit (all injected at build time).

## Filtering Files

The `--filter` flag uses **glob patterns** with `*` wildcards against filenames (not full paths):

```bash
# Match all .hcl files
hcl-linter lint ./ --filter "*.hcl"

# Match multiple patterns (OR logic)
hcl-linter lint ./ --filter "*.hcl" --filter "*.tf"

# Exact filename match
hcl-linter lint ./ --filter "terragrunt.hcl"

# Prefix/suffix patterns
hcl-linter lint ./ --filter "test*.hcl"
```

**How it works:**

- Filters match against `filepath.Base()` (filename only, not full path)
- `*` is the only wildcard supported (simple prefix/suffix matching around `*`)
- Multiple `--filter` flags use OR logic
- Files without a matching config are warned and skipped

### Advanced Filtering with Shell Pre-filtering

For more complex filtering scenarios (recursive patterns, exclusions, etc.), combine hcl-linter with shell commands:

```bash
# Recursive search with find
find ./infra -name "*.hcl" -o -name "*.tf" | xargs hcl-linter lint

# Exclude specific files
for f in *.hcl; do
  case "$f" in
    terragrunt.hcl) continue ;;
    *) hcl-linter lint "$f" ;;
  esac
done

# Exclude multiple patterns
find . -type f \( -name "*.hcl" -o -name "*.tf" \) ! -name "*test*" | xargs hcl-linter lint
```

## Concurrency

Defaults to CPU count. Override with `--concurrency <n>` or
`HCL_LINTER_MAX_CONCURRENCY` (the env var wins over the flag). Higher
values trade memory for throughput on large trees.

## Coloured output

Output is colourised when stdout is an interactive terminal. Control with
the global `--color` flag:

- `--color=auto` (default) - on when stdout is a TTY, `NO_COLOR` is unset, and `TERM` is not `dumb`
- `--color=always` - force colour on (use when piping into a colour-aware pager, e.g. `less -R`)
- `--color=never` - disable colour entirely

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

- `echo $NO_COLOR` - any non-empty value disables colour in auto mode. Unset with `unset NO_COLOR` or pass `--color=always`.
- `echo $TERM` - `dumb` disables colour. Expected values: `xterm-256color`, `screen-256color`, `tmux-256color`, etc.
- `[ -t 1 ] && echo tty || echo not-tty` - confirms whether stdout is a TTY.
- Use `--color=always` to force colour regardless of detection.

**Colour appears garbled in a pager (`^[[31m...`):** the pager isn't
passing ANSI codes through. Use `less -R` or set `PAGER="less -R"`.

**Colour bleeding into redirected files / logs:** use `--color=never`,
or don't pass `--color=always` - auto mode already strips colour for
non-TTY output.

## Environment variables

- `HCL_LINTER_CONFIG_DIR` - Path to custom config directory (priority 2; see [configuration.md](configuration.md#config-source-precedence))
- `HCL_LINTER_MAX_CONCURRENCY` - Max number of concurrent workers (overrides `--concurrency` flag)
- `NO_COLOR` - When set to any non-empty value, disables coloured output in `--color=auto` mode (see https://no-color.org)

## Exit codes

| Code | Meaning                                                                                                                       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------- |
| `0`  | Success (no issues, or `lint` completed regardless)                                                                           |
| `1`  | Issues found (`check`), config validation failed (`validate-config`), or `fix --dry-run` detected files that would be changed |
| `2`  | Configuration error (missing/unparseable config)                                                                              |
