# Configuration

`hcl-linter` is configured through one or more `.hcl` files in a
`.hcl-linter/` directory. Config files are HCL (the same format as the
files being linted) and are matched to targets by filename.

## Contents

1. [Directory layout](#directory-layout)
2. [Config source precedence](#config-source-precedence)
3. [Config file matching](#config-file-matching)
4. [Per-directory overrides](#per-directory-overrides)
5. [Config inheritance (`extends`)](#config-inheritance-extends)
6. [Full schema example](#full-schema-example)
7. [Validating your config](#validating-your-config)

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

**Full override:** user config completely replaces project configs - no
merging across source locations. Within a single config directory, use
`extends` for inheritance (see below).

The resolved config source is printed at startup. If no user config is
found and the linter falls back to project defaults, a warning is printed.

## Config file matching

For each target HCL file, the linter:

1. Tries an exact filename match: `terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`
2. Falls back to `.hcl-linter/default.hcl` if present
3. Skips the file with a warning if neither exists

A matched config may use `extends` to inherit from another config in the
same directory (see below).

Use `fix --format` to apply opinionated default formatting without
requiring any config. See [cli.md](cli.md#fix---format-flag) for details.

## Per-directory overrides

A `.hcl-linter/` directory placed anywhere in the source tree overrides
rules for files beneath it. Useful in monorepos where different services
or subprojects have different conventions.

**How it works:** for each target file, the linter walks upward from the
file's directory looking for the closest `.hcl-linter/` that contains a
usable config (either `<basename>.hcl` or `default.hcl`). That config wins.
If nothing is found on the walk, the globally-resolved root config from
[Config source precedence](#config-source-precedence) is used as the
fallback.

**Closer wins - there is no implicit cross-directory merging.** The nested
config replaces the root config wholesale for matching files. If you want
to share rules between the root and a nested config, use `extends` inside
the nested config (relative paths like `extends = "../default"` work),
or duplicate the shared rule blocks.

**Example - monorepo with one service that uses a stricter block order:**

```
repo/
├── .hcl-linter/
│   └── default.hcl             # Root rules: standard order
├── services/
│   ├── payments/
│   │   ├── .hcl-linter/
│   │   │   └── terragrunt.hcl  # Stricter order for the payments service
│   │   └── terragrunt.hcl
│   └── orders/
│       └── terragrunt.hcl      # Uses the root config (no override here)
```

When linting `services/payments/terragrunt.hcl` the nested
`services/payments/.hcl-linter/terragrunt.hcl` wins. The root config is
used unchanged for every other file.

**Bounding:** the walk is bounded by the parent of the root
`.hcl-linter/` directory. Files outside that subtree use the root config
directly and do not trigger a walk.

**Validating every config in a tree:** use
`hcl-linter validate-config --recursive [path]` to check every
`.hcl-linter/` found under the target path. See
[cli.md → `validate-config`](cli.md#validate-config-command).

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

**Example - shared base with per-file overrides:**

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
subcommand and startup warnings catch these - see
[cli.md → `validate-config`](cli.md#validate-config-command).
