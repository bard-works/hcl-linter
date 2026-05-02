# Demo: block_order rule — VALID
# Blocks appear in correct order:
#   include → locals → terraform → dependency → inputs

include "root" {
  path = find_in_parent_folders()
}

locals {
  name = "test"
}

terraform {
  source = "git::https://github.com/example/vpc.git"
}

dependency "vpc" {
  config_path = "../vpc"
}

inputs = {
  vpc_id = dependency.vpc.outputs.id
}
