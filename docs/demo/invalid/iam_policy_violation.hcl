# Pure example: IAM policy module (violations)
# Demonstrates: wrong block order, bad names, inline arrays, blank lines.

terraform {
  source = "git::https://github.com/terraform-aws-modules/terraform-aws-iam.git//modules/iam-policy?ref=v5.0.0"
}


dependency "s3-bucket" {
  config_path = "../s3-bucket"
}

include "root" {
  path = find_in_parent_folders()
}

locals {
  policyName = "s3-read-only"
  region     = "us-east-1"
}

inputs = {
  name        = local.policyName
  path        = "/"
  description = "Read-only access to S3 bucket"
  policy_statements = [
    { actions = ["s3:GetObject", "s3:ListBucket"], resources = ["arn:aws:s3:::my-bucket/*"] }
  ]
}
