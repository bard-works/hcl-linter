# Demo: dependency_outputs rule - VIOLATION
# Referencing non-existent output "nonexistent" from vpc dependency.
# Also: circular dependency (vpc references this module back).

dependency "vpc" {
  config_path = "./mock-vpc"
}

dependency "app" {
  config_path = "./mock-app"
}

inputs = {
  vpc_id     = dependency.vpc.outputs.vpc_id
  bad_output  = dependency.vpc.outputs.nonexistent
  cidr        = dependency.vpc.outputs.vpc_cidr
}
