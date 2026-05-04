# Demo: remote_state rule - VIOLATION
# remote_state block missing required "backend" attribute.

terraform {
  remote_state {
    config = {
      bucket = "my-tf-state"
      key    = "terraform.tfstate"
      region = "us-east-1"
    }
  }
}
