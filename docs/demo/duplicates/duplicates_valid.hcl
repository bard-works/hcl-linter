# Demo: duplicates rule — VALID
# All dependency blocks have unique labels.

dependency "vpc" {
  config_path = "../vpc"
}

dependency "db" {
  config_path = "../database"
}

terraform {
  source = "git::https://github.com/example/app.git"
}
