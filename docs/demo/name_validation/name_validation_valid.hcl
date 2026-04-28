# Demo: name_validation rule — VALID
# All block labels use valid snake_case format.

dependency "my_vpc" {
  config_path = "../vpc"
}

dependency "db_primary" {
  config_path = "../database"
}
