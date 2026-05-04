# Demo: required_fields rule - VALID
# All include blocks have `expose = true`.

include "root" {
  path   = find_in_parent_folders()
  expose = true
}

include "vpc" {
  path   = "../vpc"
  expose = true
}
