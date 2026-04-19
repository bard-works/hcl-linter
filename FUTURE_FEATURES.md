# Future Features

Planned features for hcl-linter, in priority order.

## High Priority

### 1. Terragrunt Function Embedding (IMPLEMENTED)
Validate Terragrunt function calls in expressions.

- `find_in_parent_folders_exists` - Check that the file being searched for exists in parent directories
- `get_env_has_default` - Warn when `get_env()` is called without a default value

**Status:** ✅ Implemented

### 1.1. Terragrunt Path Validation
Validate that referenced paths in Terragrunt blocks actually exist.

- `dependency_path_exists` - Check `dependency.config_path` exists
- `include_path_exists` - Check `include.path` exists
- `remote_state_config` - Check `terraform.remote_state` has `backend`

**Example config:**
```hcl
rules {
  terragrunt {
    enabled                = true
    dependency_path_exists = true
    include_path_exists    = true
    remote_state_config    = true
  }
}
```

**Status:** ✅ Implemented

### 2. Terraform Block Validation
Validate `terraform` blocks for completeness and correctness.

- Enforce required `source` field
- Validate `version`/`required_version` constraint format
- Check `extra_arguments` block structure
- Warn about deprecated fields

**Status:** ✅ Implemented

### 3. Key-Value Validation Rules
Add configurable attribute-level linting.

- `key_case` - Enforce naming convention (camelCase, snake_case, kebab-case)
- `value_pattern` - Regex validation for specific attribute values (e.g., AWS region format)
- `disallowed_keys` - Blocklist certain attributes that shouldn't exist

**Status:** ✅ Implemented

### 4. Count/ForEach Validation
Detect potential issues with count and for_each expressions.

- Detect dangling `count = 0` or `for_each = {}` patterns
- Warn about potentially unintended empty iterations
- Check for count/for_each conflicts

**Status:** ✅ Implemented

### 5. Nested Block Ordering
Extend `block_order` to support nested blocks.

- Example: enforce `before_hook` ordering inside `terraform` blocks
- Support dot notation: `terraform.before_hook`
- Configurable per block type

**Status:** ✅ Implemented

## Medium Priority

### 6. Dependency Output Validation
Validate `dependency.*.outputs.*` references by walking the dependency chain.

- Find `dependency` block definition → get `config_path`
- Locate `.tf` files with `output` blocks in that module
- Validate output name AND type (e.g., number for string = wrong)
- Support `.mock-outputs.json` for development

**Status:** ✅ Implemented

### 7. Config Inheritance (extends)
Allow configs to inherit from base configs.

```hcl
extends = "default"

rules {
  block_order {
    order = ["include", "locals", "terraform"]
  }
}
```

**Status:** ✅ Implemented

### 8. JSON Schema for Config
~~JSON support was removed — this item no longer applies.~~ The typo/misconfiguration
detection goal is covered by `validate-config` (item 12, implemented).
IDE autocomplete via HCL language server could be a separate future item.

### 9. IDE Integration
Improve editor support.

- `--format json` for machine-readable output
- `--format github-annotations` for GitHub Actions
- SARIF output format support
- LSP mode for in-editor linting

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

### 12. Config Validation Rules
Lint the linter config itself.

- Warn about unused config keys
- Warn about conflicting rules
- Suggest missing common keys

**Status:** ✅ Implemented — `hcl-linter validate-config` detects unknown rule blocks
and enabled rules with missing required fields. Warnings also surface at startup during
`lint`/`check`/`fix`.

### 13. Interactive Fix Mode
Prompt for each fix individually.

- `--interactive` flag
- Review changes before applying
- Skip specific fixes

### 14. Fix Dry-Run / Diff Output
Show what `fix` would change without writing files.

- `hcl-linter fix ./ --dry-run` prints a unified diff to stdout
- Useful in CI to surface formatting drift without auto-applying changes
- Exit non-zero if any changes would be made (enforces "committed files must be formatted")

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
