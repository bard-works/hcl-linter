# Demo: array_format rule — VALID
# Arrays are already in multiline format.
# Single-item arrays may stay inline (threshold is 2 by default).

locals {
  regions = [
    "us-east-1",
    "us-west-2",
    "eu-west-1",
  ]
  single = ["only-one"]
}
