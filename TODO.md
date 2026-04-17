# HCL Linter - Terraform Block Validation

## Status: IN PROGRESS

### Accomplished

1. **Config Implementation** (`internal/config/loader.go`)
   - Added `TerraformBlockConfig` struct with fields:
     - Enabled, SourceRequired, VersionFormat, ExtraArgumentsValid, NoDeprecatedFields
   - Added HCL config parsing for terraform_block rules
   - Added tests in `loader_test.go`

2. **Linter Implementation** (`internal/linter/linter.go`)
   - Added validation checks:
     - `checkTerraformSourceRequired` - validates 'source' attribute exists
     - `checkTerraformVersionFormat` - validates version string format
     - `checkTerraformExtraArguments` - validates extra_arguments block structure
     - `checkTerraformDeprecatedFields` - warns about deprecated fields/blocks

3. **Tests** (`internal/linter/linter_test.go`)
   - Added test cases for all four validation rules

4. **Fixed Parser Bug** (`internal/ast/parser.go`)
   - Fixed `GetBlockNestedBlocks` to handle block labels properly by adding `LabelNames: []string{"name"}` to schema

### Failing Tests

1. **TestTerraformExtraArguments**
   - `extra_arguments_with_name`: Reports false positive "should have 'arguments' or nested blocks" even when nested blocks exist
   - Root cause: When using label schema, nested blocks aren't being detected via `sibBody.Blocks`
   - The nested block detection using type assertion to `*hclsyntax.Body` returns empty blocks for blocks that were parsed with a label schema

2. **TestTerraformDeprecatedFields**
   - `deprecated_terraform_field`: Not detecting nested `terraform` block inside `terraform` block
   - `deprecated_before_hook_block`: Not detecting `before_hook` block
   - `deprecated_after_hook_block`: Not detecting `after_hook` block
   - Root cause: Same label schema issue - `GetBlockNestedBlocks` uses label schema so non-labeled blocks aren't found

### Issues to Fix

1. **Duplicate block detection in checkTerraformExtraArguments**
   - Current deduplication using `TypeRange.String()` may not work correctly
   - Need to verify ranges are actually equal

2. **Nested block detection for deprecated fields**
   - Need to query nested blocks without label schema for before_hook, after_hook, and nested terraform blocks

3. **Label vs no-label block schema issue**
   - When HCL parser uses label schema, nested blocks within that block are not accessible via `*hclsyntax.Body.Blocks`
   - Need alternative approach to detect nested blocks

### Files Modified

- `internal/config/loader.go` - Config struct and parsing
- `internal/config/loader_test.go` - Config tests
- `internal/linter/linter.go` - Validation logic
- `internal/linter/linter_test.go` - Test cases
- `internal/ast/parser.go` - Fixed GetBlockNestedBlocks

### Next Steps

1. Debug why nested blocks aren't detected inside extra_arguments with label
2. Fix deprecated fields detection to work with both labeled and unlabeled blocks
3. Run full test suite to verify all tests pass
4. Update SPEC.md with terraform_block rule documentation