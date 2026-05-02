# Demo: required_fields rule — VIOLATION
# include blocks must have `expose = true`.
# The fix adds the missing attribute.

include "root" {
  path = find_in_parent_folders()
}

include "vpc" {
  path = "../vpc"
}
