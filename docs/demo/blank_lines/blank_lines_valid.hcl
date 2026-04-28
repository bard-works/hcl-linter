# Demo: blank_lines rule — VALID
# At most one blank line between items within blocks.

locals {
  name = "test"

  region = "us-east-1"

  tags = {
    env = "prod"
    team = "platform"
  }
}

dependency "vpc" {
  config_path = "../vpc"
}
