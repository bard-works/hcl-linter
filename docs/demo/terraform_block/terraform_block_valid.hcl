# Demo: terraform_block rule — VALID
# - source is set
# - No deprecated fields
# - Valid version format

terraform {
  source = "git::https://github.com/example/vpc.git"

  required_version = ">= 1.0.0"
}
