rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "dependency", "inputs"]
  }

  array_format {
    enabled            = false
    multiline_threshold = 2
  }

  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = ["include", "dependency"]
  }

  duplicates {
    enabled = true
    blocks = ["locals", "dependency", "include"]
  }

  required_fields {
    include {
      expose = true
    }
  }

  blank_lines {
    enabled        = true
    within_blocks = true
  }
}