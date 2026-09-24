# Demo: hcl_functions rule - VALID
# get_env has a default; find_in_parent_folders names a file that exists in a
# parent directory (docs/demo/root.hcl).

locals {
  env   = get_env("MY_SECRET", "default")
  root  = find_in_parent_folders("root.hcl")
  port  = get_env("PORT", "8080")
}
