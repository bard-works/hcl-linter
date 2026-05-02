# Demo: dependency_paths rule — VALID
# config_path points to existing directories.
# (In demo, assumes ../vpc/ and ../database/ exist.)

dependency "vpc" {
  config_path = "../vpc"
}

dependency "db" {
  config_path = "../database"
}
