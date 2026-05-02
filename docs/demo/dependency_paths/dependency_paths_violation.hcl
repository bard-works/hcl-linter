# Demo: dependency_paths rule — VIOLATION
# config_path points to non-existent directory.

dependency "vpc" {
  config_path = "../non-existent-vpc"
}

dependency "db" {
  config_path = "/tmp/does-not-exist"
}
