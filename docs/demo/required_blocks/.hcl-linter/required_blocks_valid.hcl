rules {
  required_blocks {
    required {
      type  = "terraform"
      count = "at_least_one"
      error = "terraform block is required but missing"
    }
  }
}
