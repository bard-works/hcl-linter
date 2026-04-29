rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "dependency", "inputs"]
  }

  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_-]*$"
    blocks  = ["include", "dependency"]
  }

  array_format {
    enabled             = true
    multiline_threshold = 2
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }
}
