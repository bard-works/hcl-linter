# Demo: name_validation rule — VIOLATION
# Dependency label "my-vpc" contains hyphen, violating default pattern:
#   ^[a-z][a-z0-9_]*$
# The fix replaces hyphens with underscores.

dependency "my-vpc" {
  config_path = "../vpc"
}

dependency "db- primary" {
  config_path = "../database"
}
