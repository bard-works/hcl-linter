# Demo: hcl_functions rule - VIOLATION
# get_env without default value, find_in_parent_folders with non-existent file arg.

locals {
  env   = get_env("MY_SECRET")
  root  = find_in_parent_folders("nonexistent.hcl")
  safe  = get_env("PORT", "8080")
}
