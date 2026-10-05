# Demo: terraform_block rule - VIOLATION
# - source is empty (source_required)
# - Uses deprecated before_hook block
# - Invalid required_version constraint

terraform {
  source = ""

  required_version = "latest"

  before_hook "apply" {
    commands = ["apply"]
  }
}
