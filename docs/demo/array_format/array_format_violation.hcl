# Demo: array_format rule - VIOLATION
# Inline array with 3 items should be multiline.
# The rule converts inline arrays with 2+ items to multiline format.

locals {
  regions = ["us-east-1", "us-west-2", "eu-west-1"]
  tags    = ["prod", "monitoring"]
}
