# Demo: duplicates rule - VIOLATION
# Two dependency blocks with same label "vpc" - duplicate.

dependency "vpc" {
  config_path = "../vpc"
}

dependency "vpc" {
  config_path = "../vpc-prod"
}

terraform {
  source = "git::https://github.com/example/app.git"
}
