# Future Features

Planned features for hcl-linter. Open items are grouped by priority; shipped
features are listed under **Implemented**; dropped ideas under **Deprecated**.

## Lower Priority

### 10. Git Hook Integration

Pre-commit hook installer.

- `hcl-linter install-hook` command
- Generate `.pre-commit-config.yaml` snippet

### 11. Report Generation

Generate lint reports in various formats.

- HTML report with severity breakdown
- SARIF for GitHub Security tab integration
- JSON structured output for CI dashboards

### 13. Interactive Fix Mode

Prompt for each fix individually.

- `--interactive` flag
- Review changes before applying
- Skip specific fixes

### 16. Rule Severity Override

Let users promote warnings to errors or demote errors to warnings per rule in config.

- `severity = "error"` / `"warning"` field on any rule block
- Enables gradual adoption: introduce a rule as warning first, promote to error once the codebase is clean
- `--strict` flag treats all warnings as errors globally

### 17. Ignore Comments

`# hcl-linter:ignore` or `# hcl-linter:disable block_order` inline in HCL files
to suppress specific rules on a block or file. Essential for escape hatches
without changing config. Very common in linters (eslint-disable, golangci
nolint).


## Implemented

Original priority numbering preserved for stable reference in history and PRs.

### 18. Rule Statistics / Explain ✅

`hcl-linter explain [rule-name]` prints summary, severity, config fields, and
example violations. Without an argument it lists all rules in a tabwriter-aligned
table. Implemented via `Doc() RuleDoc` on the `Rule` interface (compiler-enforced
for all 15 rules); `cmd/hcl-linter/explain.go` handles rendering with `termcolor`
and `text/tabwriter`.

### 21. Unified Rule System ✅

`Rule` interface gains `Priority() int`; `Fixer.Fix` now mutates `ctx.Content`
in place and returns `(int, error)` — the int is the actual number of logical
edits, not a binary changed flag. All rules self-register via `func init() {
Register(...) }` into a global `DefaultRegistry`; `engine.New()` no longer
maintains a manual list. `engine.FixFile` and `engine.FormatFixFile` run a
shared `runFixPipeline` that calls `registry.Sorted()` (stable sort by
priority) and re-parses via `refreshContext` after each mutating rule.
`FormatFixFile` passes a `defaultFormatConfig()` so only the three format rules
fire. Also fixed a latent bug in `formatMultilineArray` / `fixMultilineArray`
where key extraction included the `=` sign (producing `arr = = [` for
top-level attributes); the new stricter re-parse surfaced it.
See `internal/rules/rule.go`, `registry.go`, and `internal/engine/engine.go`.

### 1. Terragrunt Function Embedding ✅

Validate Terragrunt function calls in expressions.

- `find_in_parent_folders_exists` - Check that the file being searched for exists in parent directories
- `get_env_has_default` - Warn when `get_env()` is called without a default value

### 1.1. Terragrunt Path Validation ✅

Validate that referenced paths in Terragrunt blocks actually exist.

- `dependency_path_exists` - Check `dependency.config_path` exists
- `include_path_exists` - Check `include.path` exists
- `remote_state_config` - Check `terraform.remote_state` has `backend`

### 2. Terraform Block Validation ✅

Validate `terraform` blocks for completeness and correctness.

- Enforce required `source` field
- Validate `version`/`required_version` constraint format
- Check `extra_arguments` block structure
- Warn about deprecated fields

### 3. Key-Value Validation Rules ✅

Add configurable attribute-level linting.

- `key_case` - Enforce naming convention (camelCase, snake_case, kebab-case)
- `value_pattern` - Regex validation for specific attribute values (e.g., AWS region format)
- `disallowed_keys` - Blocklist certain attributes that shouldn't exist

### 4. Count/ForEach Validation ✅

Detect potential issues with count and for_each expressions.

- Detect dangling `count = 0` or `for_each = {}` patterns
- Warn about potentially unintended empty iterations
- Check for count/for_each conflicts

### 5. Nested Block Ordering ✅

Extend `block_order` to support nested blocks.

- Example: enforce `before_hook` ordering inside `terraform` blocks
- Support dot notation: `terraform.before_hook`
- Configurable per block type

### 6. Dependency Output Validation ✅

Validate `dependency.*.outputs.*` references by walking the dependency chain.

- Find `dependency` block definition → get `config_path`
- Locate `.tf` files with `output` blocks in that module
- Validate output name AND type (e.g., number for string = wrong)
- Support `.mock-outputs.json` for development

### 7. Config Inheritance (`extends`) ✅

Allow configs to inherit from base configs.

```hcl
extends = "default"

rules {
  block_order {
    order = ["include", "locals", "terraform"]
  }
}
```

### 12. Config Validation Rules ✅

`hcl-linter validate-config` detects unknown rule blocks and enabled rules
with missing required fields. Warnings also surface at startup during
`lint`/`check`/`fix`.

### 14. Fix Dry-Run / Diff Output ✅

`hcl-linter fix <path> --dry-run` runs the fix pipeline without writing,
prints a unified diff per file that would change, and exits non-zero if
any diff is produced. Composes with `--format`. Uses
`github.com/hexops/gotextdiff` for unified-diff generation. See
`cmd/hcl-linter/diff.go` and [docs/cli.md → `fix --dry-run` flag](docs/cli.md#fix---dry-run-flag).

### 15. Per-Directory Config Override ✅

A `.hcl-linter/` directory anywhere in the source tree overrides rules for
files beneath it. For each target file, the loader walks upward from the
file's directory looking for the closest `.hcl-linter/` that contains a
usable config (`<basename>.hcl` or `default.hcl`); that config wins
wholesale. If nothing is found, the globally-discovered root config is
used. The walk is bounded by the parent of the root `.hcl-linter/` so
files outside the project tree skip it entirely.

No implicit cross-directory merging — closer wins. Users share rules via
`extends` within a nested dir or by duplicating blocks. `validate-config
--recursive` walks a path and validates every nested `.hcl-linter/` found.
See [docs/configuration.md → Per-directory overrides](docs/configuration.md#per-directory-overrides)
and `internal/config/loader.go`.

### 20. `init` Command — Bootstrap Config for a Directory ✅

`hcl-linter init [path]` walks the target for `.hcl` / `.tf` files (skipping
hidden dirs via `findHCLFiles`), groups them by unique basename, and writes:

- `.hcl-linter/default.hcl` — baseline with `block_order`, `array_format`,
  and `blank_lines` enabled with safe defaults. Passes `validate-config`
  out of the box.
- `.hcl-linter/<name>.hcl` per unique basename — a thin
  `extends = "default"` override with an empty `rules {}` starter block.

Flags: `--dry-run` prints the proposed layout to stdout without writing;
`--force` overwrites an existing non-empty `.hcl-linter/` (the default
refuses and lists what was found). See
[docs/cli.md → `init`](docs/cli.md#init) and `cmd/hcl-linter/init.go`.

### 19. Coloured Terminal Output ✅

Colourise lint output for readability in interactive terminals.

- Red for errors, yellow for warnings, green for "clean" summary lines
- Highlight file paths, line:column positions, and rule names distinctly
- Auto-detect TTY: colour on when stdout is a terminal, off when piped/redirected
- `--color=auto|always|never` flag to override detection
- Respect `NO_COLOR` env var (https://no-color.org/)

Implementation: `internal/termcolor` wraps `fatih/color` with semantic
helpers (Error/Warning/Success/Path/Rule/Location). Global `--color` flag
with `auto` (default), `always`, `never`. Auto honours `NO_COLOR` and
`TERM=dumb`; explicit `always` overrides both. See
[docs/cli.md → Coloured output](docs/cli.md#coloured-output) for debugging.

---

## Deprecated

Ideas kept for history. Do not reintroduce without revisiting the rationale.

### 8. JSON Schema for Config ❌

**Dropped.** JSON support for config files was intentionally removed (HCL
only). The typo/misconfiguration detection goal this item was meant to
address is covered by `validate-config` (see Implemented #12). IDE
autocomplete via an HCL language server is the remaining gap, but would be
pursued as a separate initiative rather than a JSON schema.

### 9. IDE Integration ❌

**Dropped.** Was a catch-all for unrelated ideas (machine-readable output
formats, GitHub annotations, SARIF, LSP mode). Superseded by narrower,
independently-scoped items: report formats land under Lower Priority #11,
and an LSP mode would be a separate future proposal.
