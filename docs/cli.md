# CLI Reference

## Contents

1. [Commands](#commands)
2. [Global flags](#global-flags)
3. [`lint`](#lint)
4. [`check`](#check)
5. [`fix`](#fix)
   - [`fix --format` flag](#fix---format-flag)
   - [`fix --dry-run` flag](#fix---dry-run-flag)
6. [Report formats (`--output`)](#report-formats---output)
7. [`validate-config`](#validate-config-command)
8. [`init`](#init)
9. [`explain`](#explain)
10. [`version`](#version)
11. [Target file filtering (`--filter`)](#filtering-files)
12. [Concurrency](#concurrency)
13. [Coloured output](#coloured-output)
14. [Environment variables](#environment-variables)
15. [Exit codes](#exit-codes)

## Commands

```bash
# Lint all files under a path
hcl-linter lint ./

# Lint specific files or several paths at once
hcl-linter lint ./payments/integration/iam-policy/terragrunt.hcl
hcl-linter check live/prod live/staging modules/vpc/main.tf

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

`lint`, `check` and `fix` accept one or more paths (files or directories).

**Config is required** for `lint`, `check` and `fix` (without `--format`):
with no `.hcl-linter/` found they exit `2` and suggest `hcl-linter init`,
rather than passing silently. Files without a matching config file (no
`.hcl-linter/<name>.hcl` and no `.hcl-linter/default.hcl`) are skipped.
Files that fall back to `default.hcl` trigger one warning per filename.
`fix --format` needs no config at all.

## Global flags

```
--config-source, -c   Config source path (overrides env/defaults)
--filter              Filter files by name pattern (glob, repeatable)
--concurrency         Max concurrent workers (default: CPU count)
--verbose, -v         Show detailed output
--color               Coloured output: auto (default), always, never
--include-hidden      Include files in hidden directories (dirs starting with .)
--version             Print version information
```

Directory targets skip hidden directories (unless `--include-hidden`) and
`.terraform.lock.hcl`, which `terraform init` generates. Pass a file
explicitly to lint it anyway.

## `lint`

Lints every matching file under the given paths and prints all issues.
Exits `0` regardless of issue count - use `check` if you want a non-zero
exit on issues.

## `check`

Same rule set as `lint`, but exits `1` if any error-severity issue is
found (warnings are reported but do not fail the run). Intended for CI.

Findings can be silenced in the file itself with
`# hcl-linter:ignore <rule>` comments - see
[configuration.md → Inline suppression](configuration.md#inline-suppression).

## Report formats (`--output`)

`lint` and `check` take `--output` / `-o`:

| Format       | Use                                                                                 |
| ------------ | ----------------------------------------------------------------------------------- |
| `text`       | Default. `path:line:col: [severity] rule: message` plus a summary line              |
| `json`       | Scripts. `{"version":1,"issues":[{file,line,column,severity,rule,source,message}],"summary":{...}}` |
| `sarif`      | GitHub code scanning (`github/codeql-action/upload-sarif`) and other SARIF 2.1.0 tools |
| `checkstyle` | reviewdog (`-f=checkstyle`), Jenkins, GitLab plugins                                |

Only the report goes to stdout, so `hcl-linter check . -o sarif > hcl.sarif`
is safe; the config banner and warnings stay on stderr. Exit codes are the
same for every format. Paths are relative to the working directory when the
target is beneath it, except in SARIF, which uses absolute `file://` URIs:
GitHub code scanning reads relative URIs from the repository root, so they
would point at the wrong file when hcl-linter runs in a subdirectory, and
`upload-sarif` turns absolute ones into repository-relative paths.

## `fix`

Runs the fix pipeline (BlockOrder → NameValidation → RequiredFields →
ArrayFormat → BlankLines) against each matching file and writes the
result back to disk. Rules without a `Fix` implementation are skipped, as
are rules disabled for a file with `# hcl-linter:ignore-file <rule>`.
Each fix is re-parsed before it is written; a file that can't be parsed
(before or after fixing) is left untouched and the run exits `3`.

Rules are only applied when enabled in the matched config, unless
`--format` is given.

### `fix --format` flag

The `--format` flag applies opinionated default formatting to all HCL
files without requiring any config to be present:

- **Block order:** `include → locals → terraform → remote_state → generate → dependencies → dependency`,
  the same as the `builtin:terragrunt` preset, so `fix --format` and `check` never disagree
- **Array format:** converts inline arrays with 2+ items to multiline
- **Blank lines:** removes blank lines at block boundaries, collapses consecutive blank lines

- **Canonical HCL formatting:** the same formatter `terraform fmt` uses
  (`hclwrite.Format`) - attribute alignment, indentation, spacing

`.hcl-linter/` is never read in this mode: every `.hcl` / `.tf` file under
the target path is formatted with the defaults above, whether or not a
config exists. `--filter` and `--include-hidden` still apply. Running it
twice produces no further changes, so `fix --format --dry-run` is a safe
CI gate after a local `fix --format`.

This is useful for one-off formatting or bootstrapping a codebase before
introducing config-based rules.

### `fix --dry-run` flag

Runs the fix pipeline but prints a unified diff per changed file instead
of writing. Exits `1` if any file would change. Composes with `--format`.

```bash
hcl-linter fix ./ --dry-run
hcl-linter fix ./ --format --dry-run   # config-free formatting check
```

Not the same as `check`: `check` reports rule violations, `--dry-run`
reports byte-level drift, including formatting a rule's check doesn't
describe line by line. Use `--dry-run` in CI to fail
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

Walks the target directory for `.hcl` / `.tf` files (skipping hidden dirs
and `.terraform.lock.hcl`),
groups them by unique basename, then writes:

- `.hcl-linter/default.hcl` - extends the recommended preset:
  `builtin:terragrunt` when the project contains `terragrunt.hcl` or
  `root.hcl`, otherwise `builtin:terraform`
  (see [rules.md → Presets](rules.md#presets)).
- `.hcl-linter/<name>.hcl` per unique target name without its extension
  (`main.tf` → `main.hcl`, which is how configs are matched) - a thin
  `extends = "default"` override with an empty `rules {}` block, ready for
  per-filename customisation. In a Terragrunt project, `.tf` names extend
  `builtin:terraform` instead.

**Flags:**

- `--dry-run` - print the proposed layout and file contents to stdout; no
  files are written.
- `--force` - overwrite existing `.hcl-linter/` contents. Without this flag,
  `init` refuses when the target `.hcl-linter/` directory is non-empty and
  lists what it found.

The generated layout passes `validate-config` unchanged; `check .` right
after `init` reports real findings only.

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

Prints version, build date, and git commit (`hcl-linter --version` is
equivalent). Release builds inject these at build time; `go install` builds
read them from Go's embedded build info.

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
- Patterns use Go's [`filepath.Match`](https://pkg.go.dev/path/filepath#Match)
  syntax: `*`, `?`, and character classes like `[a-z]`
- Multiple `--filter` flags use OR logic
- Files without a matching config are warned and skipped

### Advanced Filtering with Shell Pre-filtering

For more complex filtering scenarios (recursive patterns, exclusions, etc.), combine hcl-linter with shell commands:

```bash
# Recursive search with find
find ./infra \( -name "*.hcl" -o -name "*.tf" \) -print0 | xargs -0 hcl-linter lint

# Exclude specific files
find . -name "*.hcl" ! -name terragrunt.hcl -print0 | xargs -0 hcl-linter lint

# Exclude multiple patterns
find . -type f \( -name "*.hcl" -o -name "*.tf" \) ! -name "*test*" | xargs hcl-linter lint
```

## Concurrency

Defaults to CPU count. Override with `--concurrency <n>` or
`HCL_LINTER_MAX_CONCURRENCY` (the flag wins over the env var). Higher
values trade memory for throughput on large trees.

## Coloured output

Output is colourised when stdout is an interactive terminal. Control with
the global `--color` flag:

- `--color=auto` (default) - on when both stdout and stderr are TTYs, `NO_COLOR` is unset, and `TERM` is not `dumb`
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
- `HCL_LINTER_MAX_CONCURRENCY` - Max number of concurrent workers (used when `--concurrency` is not set)
- `NO_COLOR` - When set to any non-empty value, disables coloured output in `--color=auto` mode (see https://no-color.org)

## Exit codes

| Code | Meaning                                                                                                                        |
| ---- | ------------------------------------------------------------------------------------------------------------------------------ |
| `0`  | Success (no findings; `lint` also exits 0 when it finds issues - it is report-only)                                            |
| `1`  | Findings: issues found (`check`), config validation failed (`validate-config`), or `fix --dry-run` detected files that would change |
| `2`  | Usage or configuration error (bad flag, missing path, no config directory for `lint`/`check`/`fix`)                            |
| `3`  | Execution failure: the tool itself could not process input (unparseable file, IO error, interrupted) - also for `fix`          |

Results (issues, diffs, summaries) go to stdout; diagnostics (config banner,
warnings, progress) go to stderr, so piping stdout yields machine-readable
output only.
