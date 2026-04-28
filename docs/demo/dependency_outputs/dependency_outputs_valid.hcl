# Demo: dependency_outputs rule — VALID
# All dependency output references match actual outputs in target modules.

dependency "vpc" {
  config_path = "./mock-vpc"
}

inputs = {
  vpc_id = dependency.vpc.outputs.vpc_id
  cidr   = dependency.vpc.outputs.vpc_cidr
}
