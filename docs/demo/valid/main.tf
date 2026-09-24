# Pure Terraform example: IAM policy (valid)
# Correct block order, snake_case names, multiline arrays.

terraform {
  required_version = ">= 1.0.0"
}

resource "aws_iam_policy" "s3_read" {
  name        = "s3-read-only"
  path        = "/"
  description = "Read-only access to S3 bucket"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action   = ["s3:GetObject", "s3:ListBucket"]
        Effect   = "Allow"
        Resource = ["arn:aws:s3:::my-bucket/*"]
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "attach" {
  role       = aws_iam_role.app.name
  policy_arn = aws_iam_policy.s3_read.arn
}

output "policy_arn" {
  value = aws_iam_policy.s3_read.arn
}
