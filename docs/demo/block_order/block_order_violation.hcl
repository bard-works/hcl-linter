# Demo: block_order rule — VIOLATION
# Blocks appear in wrong order. Configured order is:
#   include → locals → terraform → dependency → inputs
# Here, terraform appears before include (wrong order).

terraform {
  source = "git::https://github.com/example/vpc.git"
}

include "root" {
  path = find_in_parent_folders()
}

locals {
  name = "test"
}

dependency "vpc" {
  config_path = "../vpc"
}

inputs = {
  vpc_id = dependency.vpc.outputs.id
}
