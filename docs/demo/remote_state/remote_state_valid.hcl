# Demo: remote_state rule - VALID
# remote_state block has "backend" attribute set.

terraform {
  remote_state {
    backend = "s3"
    config = {
      bucket = "my-tf-state"
      key    = "terraform.tfstate"
      region = "us-east-1"
    }
  }
}
