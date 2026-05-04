# Demo Files for hcl-linter Rules

Each subdirectory contains **valid** and **violation** HCL files for one rule,
plus `.hcl-linter/` configs named after the demo files.

## Structure

```
docs/demo/
├── valid/                              # Pure/vanilla examples (valid only)
│   ├── .hcl-linter/
│   │   └── default.hcl                 # Config for .tf files (fallback)
│   ├── iam_policy_valid.hcl            # Clean Terragrunt IAM policy
│   └── iam_policy_valid.tf             # Clean Terraform IAM policy (.tf)
├── invalid/                            # Violation examples
│   ├── .hcl-linter/
│   │   ├── default.hcl                 # Config for invalid_terraform.tf
│   │   ├── iam_policy_violation.hcl    # Config for .hcl violation
│   │   └── iam_policy_violation.tf.hcl # Config for .tf violation
│   ├── invalid_terraform.tf            # Multi-violation: order, camelCase, inline arrays
│   ├── iam_policy_violation.hcl        # Violations: order, naming, arrays
│   └── iam_policy_violation.tf         # Violations: order, camelCase, inline arrays
├── block_order/
│   ├── .hcl-linter/
│   │   ├── block_order_valid.hcl       # Config for valid file
│   │   └── block_order_violation.hcl   # Config for violation file
│   ├── block_order_valid.hcl           # Passes the rule
│   └── block_order_violation.hcl       # Triggers the rule
├── array_format/
│   ├── .hcl-linter/
│   ├── array_format_valid.hcl
│   └── array_format_violation.hcl
└── ...
```

## File Naming

| Pattern                                   | Purpose                             |
| ----------------------------------------- | ----------------------------------- |
| `<rule>_valid.hcl`                        | HCL file that **passes** the rule   |
| `<rule>_violation.hcl`                    | HCL file that **triggers** the rule |
| `<rule>/.hcl-linter/<rule>_valid.hcl`     | Config for valid file               |
| `<rule>/.hcl-linter/<rule>_violation.hcl` | Config for violation file           |

## Usage

Lint a demo file against its config:

```bash
hcl-linter lint docs/demo/block_order/block_order_violation.hcl --config-source docs/demo/block_order/.hcl-linter
```

Fix a violation (if rule is fixable):

```bash
hcl-linter fix docs/demo/block_order/block_order_violation.hcl --config-source docs/demo/block_order/.hcl-linter --dry-run
```

## Run Commands

### Lint (check mode)

```bash
# Fixable rules
hcl-linter lint docs/demo/block_order/ --config-source docs/demo/block_order/.hcl-linter --verbose
hcl-linter lint docs/demo/array_format/ --config-source docs/demo/array_format/.hcl-linter --verbose
hcl-linter lint docs/demo/blank_lines/ --config-source docs/demo/blank_lines/.hcl-linter --verbose
hcl-linter lint docs/demo/name_validation/ --config-source docs/demo/name_validation/.hcl-linter --verbose
hcl-linter lint docs/demo/required_fields/ --config-source docs/demo/required_fields/.hcl-linter --verbose

# Pure Terraform (.tf) examples - uses default.hcl fallback
hcl-linter lint docs/demo/valid/iam_policy_valid.tf --config-source docs/demo/valid/.hcl-linter --verbose
hcl-linter lint docs/demo/invalid/iam_policy_violation.tf --config-source docs/demo/invalid/.hcl-linter --verbose
hcl-linter lint docs/demo/invalid/invalid_terraform.tf --config-source docs/demo/invalid/.hcl-linter --verbose

# Non-fixable rules
hcl-linter lint docs/demo/required_blocks/ --config-source docs/demo/required_blocks/.hcl-linter --verbose
hcl-linter lint docs/demo/duplicates/ --config-source docs/demo/duplicates/.hcl-linter --verbose
hcl-linter lint docs/demo/count_foreach/ --config-source docs/demo/count_foreach/.hcl-linter --verbose
hcl-linter lint docs/demo/dependency_outputs/ --config-source docs/demo/dependency_outputs/.hcl-linter --verbose
hcl-linter lint docs/demo/dependency_paths/ --config-source docs/demo/dependency_paths/.hcl-linter --verbose
hcl-linter lint docs/demo/include_paths/ --config-source docs/demo/include_paths/.hcl-linter --verbose
hcl-linter lint docs/demo/hcl_functions/ --config-source docs/demo/hcl_functions/.hcl-linter --verbose
hcl-linter lint docs/demo/terraform_block/ --config-source docs/demo/terraform_block/.hcl-linter --verbose
hcl-linter lint docs/demo/key_value/ --config-source docs/demo/key_value/.hcl-linter --verbose
hcl-linter lint docs/demo/remote_state/ --config-source docs/demo/remote_state/.hcl-linter --verbose
```

### Fix (dry-run)

```bash
hcl-linter fix docs/demo/block_order/block_order_violation.hcl --config-source docs/demo/block_order/.hcl-linter --dry-run
hcl-linter fix docs/demo/array_format/array_format_violation.hcl --config-source docs/demo/array_format/.hcl-linter --dry-run
hcl-linter fix docs/demo/blank_lines/blank_lines_violation.hcl --config-source docs/demo/blank_lines/.hcl-linter --dry-run
hcl-linter fix docs/demo/name_validation/name_validation_violation.hcl --config-source docs/demo/name_validation/.hcl-linter --dry-run
hcl-linter fix docs/demo/required_fields/required_fields_violation.hcl --config-source docs/demo/required_fields/.hcl-linter --dry-run

# Pure Terraform (.tf) fix dry-run
hcl-linter fix docs/demo/invalid/iam_policy_violation.tf --config-source docs/demo/invalid/.hcl-linter --dry-run
hcl-linter fix docs/demo/invalid/iam_policy_violation.hcl --config-source docs/demo/invalid/.hcl-linter --dry-run
hcl-linter fix docs/demo/invalid/invalid_terraform.tf --config-source docs/demo/invalid/.hcl-linter --dry-run
```

## Rules Covered

| Rule                                  | Fixable | Demo Dir              |
| ------------------------------------- | ------- | --------------------- |
| `block_order`                         | Yes     | `block_order/`        |
| `array_format`                        | Yes     | `array_format/`       |
| `blank_lines`                         | Yes     | `blank_lines/`        |
| `name_validation`                     | Yes     | `name_validation/`    |
| `required_fields`                     | Yes     | `required_fields/`    |
| _multi-rule vanilla_ (`.hcl` + `.tf`) | Yes     | `valid/`              |
| `required_blocks`                     | No      | `required_blocks/`    |
| `duplicates`                          | No      | `duplicates/`         |
| `count_for_each`                      | No      | `count_foreach/`      |
| `dependency_outputs`                  | No      | `dependency_outputs/` |
| `dependency_paths`                    | No      | `dependency_paths/`   |
| `include_paths`                       | No      | `include_paths/`      |
| `hcl_functions`                       | No      | `hcl_functions/`      |
| `terraform_block`                     | No      | `terraform_block/`    |
| `key_value`                           | No      | `key_value/`          |
| `remote_state`                        | No      | `remote_state/`       |
