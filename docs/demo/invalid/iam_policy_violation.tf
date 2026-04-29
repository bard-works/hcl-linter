# Pure Terraform example: IAM policy (violations)
# Wrong block order, hyphen in resource name, inline arrays.

output "policy_arn" {
  value = aws_iam_policy.s3_read.arn
}

resource "aws_iam_policy" "s3-read" {
  name        = "s3-read-only"
  path        = "/"
  description = "Read-only access to S3 bucket"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Action = ["s3:GetObject", "s3:ListBucket"], Effect = "Allow", Resource = ["arn:aws:s3:::my-bucket/*"] }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "attach" {
  role       = aws_iam_role.app.name
  policy_arn = aws_iam_policy.s3_read.arn
}

terraform {
  required_version = ">= 1.0.0"
}
