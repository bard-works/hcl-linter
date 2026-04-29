# Pure example: IAM policy module (valid)
# Demonstrates clean HCL: correct block order, snake_case, proper formatting.

include "root" {
  path = find_in_parent_folders()
}

locals {
  policy_name = "s3-read-only"
  region      = "us-east-1"
}

terraform {
  source = "git::https://github.com/terraform-aws-modules/terraform-aws-iam.git//modules/iam-policy?ref=v5.0.0"
}

dependency "s3_bucket" {
  config_path = "../s3-bucket"
}

inputs = {
  name        = local.policy_name
  path        = "/"
  description = "Read-only access to S3 bucket"
  policy_statements = [
    {
      actions   = ["s3:GetObject", "s3:ListBucket"]
      resources = ["arn:aws:s3:::my-bucket/*"]
    },
  ]
}
