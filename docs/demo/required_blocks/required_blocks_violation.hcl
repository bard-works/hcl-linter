# Demo: required_blocks rule — VIOLATION
# terraform block is required (at_least_one) but missing.

locals {
  name = "test"
}

dependency "vpc" {
  config_path = "../vpc"
}

inputs = {
  vpc_id = dependency.vpc.outputs.id
}
