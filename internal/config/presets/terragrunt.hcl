# builtin:terragrunt - recommended rules for Terragrunt units, root.hcl and
# other shared .hcl files. Formatting plus checks for references that break
# at plan time: dependency/include paths, find_in_parent_folders, get_env,
# remote_state backend. Opinionated naming rules are left to the project.
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

  duplicates {
    enabled = true
    blocks  = ["locals", "include", "dependency", "dependencies", "terraform", "remote_state"]
  }

  dependency_paths {
    enabled = true
  }

  include_paths {
    enabled = true
  }

  hcl_functions {
    enabled                       = true
    find_in_parent_folders_exists = true
    get_env_has_default           = true
  }

  terraform_block {
    enabled               = true
    version_format        = true
    extra_arguments_valid = true
    no_deprecated_fields  = true
  }

  remote_state {
    enabled         = true
    require_backend = true
  }
}
