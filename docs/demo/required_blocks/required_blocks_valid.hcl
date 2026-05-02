# Demo: required_blocks rule — VALID
# terraform block is present as required.

terraform {
  source = "git::https://github.com/example/vpc.git"
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
