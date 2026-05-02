# Demo: blank_lines rule — VIOLATION
# Too many blank lines within blocks and object attributes.
# The fix removes excess blank lines (keeps at most one).

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
