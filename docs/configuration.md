# Configuration

`hcl-linter` is configured through one or more `.hcl` files in a
`.hcl-linter/` directory. Config files are HCL (the same format as the
files being linted) and are matched to targets by filename.

## Contents

1. [Directory layout](#directory-layout)
2. [Config source precedence](#config-source-precedence)
3. [Config file matching](#config-file-matching)
4. [Config inheritance (`extends`)](#config-inheritance-extends)
5. [Full schema example](#full-schema-example)
6. [Validating your config](#validating-your-config)

## Directory layout

```
.hcl-linter/
├── default.hcl       # Base config for all files (optional fallback)
├── terragrunt.hcl    # Rules for terragrunt.hcl
├── root.hcl          # Rules for root.hcl
└── service.hcl       # Rules for service.hcl
```

Each file is named after the target filename it configures. See
[Config file matching](#config-file-matching) for lookup rules.

## Config source precedence

Configs are loaded from the first source that exists, in this order:

| Priority | Source                      | Description                      |
| -------- | --------------------------- | -------------------------------- |
| 1        | CLI `--config-source`       | Explicit path via flag           |
| 2        | Env `HCL_LINTER_CONFIG_DIR` | Environment variable             |
| 3        | `.hcl-linter/` in cwd       | User config in current directory |
| 4        | `.hcl-linter/` in home      | User config in home directory    |
| 5        | Project's `.hcl-linter/`    | Built-in defaults (fallback)     |

**Full override:** user config completely replaces project configs — no
merging across source locations. Within a single config directory, use
`extends` for inheritance (see below).

The applied config source is printed at startup for transparency. If the
linter falls back to project defaults (no user config found), a warning is
displayed.

## Config file matching

For each target HCL file, the linter:

1. Tries an exact filename match: `terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`
2. Falls back to `.hcl-linter/default.hcl` if present
3. Skips the file with a warning if neither exists

A matched config may use `extends` to inherit from another config in the
same directory (see below).

Use `fix --format` to apply opinionated default formatting without
requiring any config. See [cli.md](cli.md#fix---format-flag) for details.

## Config inheritance (`extends`)

A config file can inherit from another config in the same directory using
the top-level `extends` attribute:

```hcl
extends = "default"

rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}
```

**Behavior:**

- `extends` takes a config name (with or without the `.hcl` extension) relative to the same directory
- Base config is loaded first; child rules override the base at the rule-block level (whole rule blocks, not individual fields)
- Unset rules in the child are inherited from the base unchanged
- Chains are supported: `child extends parent extends grandparent`
- Circular references are detected and reported as an error

**Example — shared base with per-file overrides:**

```
.hcl-linter/
├── default.hcl       # Shared base rules (block_order, blank_lines, etc.)
├── terragrunt.hcl    # extends = "default", overrides block_order
└── root.hcl          # extends = "default", adds required_blocks
```

## Full schema example

Every rule block shown here with its commonly-used fields. Any rule block
may be omitted to disable that rule; see [rules.md](rules.md) for per-rule
details.

```hcl
rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "dependency", "inputs"]
  }

  array_format {
    enabled             = true
    multiline_threshold = 2
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }

  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["include", "dependency"]
  }

  duplicates {
    enabled = true
    blocks  = ["locals", "dependency", "include"]
  }

  required_fields {
    include {
      expose = true
    }
  }

  required_blocks {
    required {
      type  = "terraform"
      count = "once"
      error = "missing terraform block"
    }
  }

  dependency_paths {
    enabled = true
  }

  include_paths {
    enabled = true
  }

  remote_state {
    enabled         = true
    require_backend = true
  }

  hcl_functions {
    enabled                       = true
    find_in_parent_folders_exists = true
    get_env_has_default           = true
  }

  terraform_block {
    enabled               = true
    source_required       = true
    version_format        = true
    extra_arguments_valid = true
    no_deprecated_fields  = true
  }

  key_value {
    enabled   = true
    key_case  = "snake_case"
    value_pattern = {
      region = "^us-[a-z]+-[0-9]+$"
    }
    disallowed = ["secret", "password"]
  }

  count_for_each {
    enabled                = true
    warn_on_count_zero     = true
    warn_on_empty_for_each = true
    warn_on_conflict       = true
  }

  dependency_outputs {
    enabled = true
  }
}
```

## Validating your config

Because config files are plain HCL, typos in rule block names (e.g.
`blokc_order`) are silently ignored by the parser. The `validate-config`
subcommand and startup warnings catch these — see
[cli.md → `validate-config`](cli.md#validate-config-command).
