# Demo: include_paths rule - VIOLATION
# path attribute points to non-existent file.

include "root" {
  path = "../missing/root.hcl"
}

include "parent" {
  path = "/tmp/does-not-exist.hcl"
}
