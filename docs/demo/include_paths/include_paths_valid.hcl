# Demo: include_paths rule — VALID
# path uses find_in_parent_folders() or points to existing files.

include "root" {
  path = find_in_parent_folders()
}

# Assuming ../shared/root.hcl exists:
# include "shared" {
#   path = "../shared/root.hcl"
# }
