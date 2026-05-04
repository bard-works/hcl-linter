# Invalid Terraform example - multiple linter violations
# 1. Wrong block order (output before resource)
# 2. camelCase output name
# 3. Inline array (should be multiline)
# 4. Bad resource type label (camelCase)

output "instanceId" {
  value = aws_instance.web.id
}

resource "aws_instance" "web" {
  ami                 = "ami-12345678"
  instance_type = "t2.micro"

  tags = { Name = "web", Environment = "dev" }
}

resource "aws_s3_Bucket" "data" {
  bucket = "my-bucket"
  acl    = "private"

  versioning {
    enabled = true
  }

  tags = ["Name:data", "Env:prod"]
}

terraform {
  required_version = ">= 1.0.0"
}
