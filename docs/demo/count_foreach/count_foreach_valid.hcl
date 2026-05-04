# Demo: count_for_each rule - VALID
# Resources use either count or for_each, not both. count > 0.

resource "aws_instance" "web" {
  count = 1
  ami   = "ami-12345"
}

resource "aws_db_instance" "db" {
  for_each = var.db_configs
  engine   = "postgres"
}
