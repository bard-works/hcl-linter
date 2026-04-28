# Demo: key_value rule — VIOLATION
# - Key "myKey" violates snake_case
# - Key "password" is in disallowed list
# - Value doesn't match required pattern

locals {
  myKey    = "value"
  password = "secret123"
  env       = "PROD"  # expecting "dev" or "prod" lowercase
}

inputs = {
  mySetting = "value"
}
