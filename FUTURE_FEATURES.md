# Future Features

Planned features for hcl-linter. Open items are grouped by priority; shipped
features are listed under **Implemented**; dropped ideas under **Deprecated**.

## Top Priority

### 20. `init` Command — Bootstrap Config for a Directory

`hcl-linter init` helps a user set up a working `.hcl-linter/` for the
current project without hand-writing HCL from scratch.

- Walk the cwd for `.hcl` / `.tf` files, skipping hidden dirs (same rules
  as `findHCLFiles`).
- Group by **unique filename** (e.g. `terragrunt.hcl`, `root.hcl`,
  `service.hcl`) — one config file per unique name is the natural mapping
  given the existing per-filename config-matching model.
- Print the detected filenames and the proposed layout, then write:
  - `.hcl-linter/default.hcl` — sensible baseline (block_order, array_format,
    blank_lines enabled with safe defaults)
  - `.hcl-linter/<name>.hcl` per unique filename — `extends = "default"`
    with an empty `rules {}` override block as a starting point
- Flags:
  - `--dry-run` — print proposed files to stdout without writing
  - `--force` — overwrite existing `.hcl-linter/` contents (default: refuse
    if the directory is non-empty, print what was found)
- Output should nudge the user toward `validate-config` and `fix --dry-run`
  as the next steps.

Design notes:

- Reuse `findHCLFiles` from `cmd/hcl-linter/main.go`; don't duplicate the
  walker.
- The default template must pass `validate-config` out of the box — all
  enabled rules need their required fields set.
- Compose cleanly with `extends` (Implemented #7): per-filename configs
  start as thin overrides, not copies of the default.
- No interactive prompts in v1 — keep it non-interactive so it's
  scriptable in CI / Makefiles. Interactive polish can come later.

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

### 15. Per-Directory Config Override

Allow a `.hcl-linter/` directory anywhere in the tree to override rules for files beneath it.

- Closer config wins; root config is the fallback
- Same `extends` mechanism for inheritance
- Useful in monorepos where different services have different conventions

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

### 18. Rule Statistics / Explain

`hcl-linter explain block_order` prints what the rule checks, what config keys
it accepts, and an example violation+fix. Helps onboarding new contributors
and debugging why a rule isn't firing.

---

## Implemented

Original priority numbering preserved for stable reference in history and PRs.

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
