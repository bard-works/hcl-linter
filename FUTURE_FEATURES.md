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
```json
{
  "rules": {
    "terragrunt": {
      "enabled": true,
      "dependency_path_exists": true,
      "include_path_exists": true,
      "remote_state_config": true
    }
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

### 4. Count/ForEach Validation
Detect potential issues with count and for_each expressions.

- Detect dangling `count = 0` or `for_each = {}` patterns
- Warn about potentially unintended empty iterations
- Check for count/for_each conflicts

### 5. Nested Block Ordering
Extend `block_order` to support nested blocks.

- Example: enforce `before_hook` ordering inside `terraform` blocks
- Support dot notation: `terraform.before_hook`
- Configurable per block type

**Status:** ✅ Implemented

## Medium Priority

### 6. Dependency Output Validation
Validate `dependency.*.outputs.*` references by walking the dependency chain.

- Find `dependency` block definition → get `source` path
- Locate `terragrunt.hcl` in that path → extract terraform source
- Parse `outputs.tf` (or `.tf` files with `output` blocks) in that module
- Verify the referenced output exists

**Example:**
```hcl
# In your terragrunt.hcl
dependency "vpc" {
  config_path = "../vpc"
}

# Reference outputs
inputs = {
  vpc_id = dependency.vpc.outputs.vpc_id  # ✓ validated
  fake  = dependency.vpc.outputs.fake_id  # ✗ not defined in ../vpc/outputs.tf
}
```

**Requirements:**
- No terraform/terragrunt execution
- No state file access
- Works with mock outputs (useful during development)

### 7. Config Inheritance (extends)
Allow configs to inherit from base configs.

```json
{
  "extends": "default",
  "rules": {
    "block_order": {
      "order": ["include", "locals", "terraform"]
    }
  }
}
```

### 8. JSON Schema for Config
Validate `.hcl-linter/` config files against a schema.

- Catch typos in rule names
- Provide IDE autocomplete support
- Validate rule configuration structure

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

### 13. Interactive Fix Mode
Prompt for each fix individually.

- `--interactive` flag
- Review changes before applying
- Skip specific fixes
