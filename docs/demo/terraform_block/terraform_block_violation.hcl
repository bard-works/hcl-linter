# Demo: terraform_block rule — VIOLATION
# - source is empty (source_required)
# - Uses deprecated before_hook block
# - Invalid version format

terraform {
  source = ""

  required_version = "1.0"

  before_hook "apply" {
    commands = ["apply"]
  }
}
