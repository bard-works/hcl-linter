# Demo: hcl_functions rule — VALID
# get_env has default, find_in_parent_folders uses no arg or existing file.

locals {
  env   = get_env("MY_SECRET", "default")
  root  = find_in_parent_folders()
  port  = get_env("PORT", "8080")
}
