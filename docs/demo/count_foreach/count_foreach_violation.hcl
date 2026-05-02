# Demo: count_for_each rule — VIOLATION
# Resource has count = 0 (will not be created) and both count + for_each set.

resource "aws_instance" "web" {
  count = 0
  ami   = "ami-12345"
}

resource "aws_db_instance" "db" {
  count     = 1
  for_each  = var.db_configs
  engine    = "postgres"
}
