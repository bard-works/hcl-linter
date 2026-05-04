# Demo: key_value rule - VALID
# - All keys use snake_case
# - No disallowed keys
# - Values match required patterns

locals {
  my_key  = "value"
  env     = "prod"
}

inputs = {
  my_setting = "value"
}
