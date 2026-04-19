rules {
  block_order {
    enabled = true
    order   = [
      "dependency",
      "include",
      "inputs",
      "locals",
      "terraform",
    ]
  }

  array_format {
    enabled            = false
    multiline_threshold = 2
  }

  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks  = [
      "dependency",
      "include",
    ]
  }

  duplicates {
    enabled = true
    blocks = [
      "dependency",
      "include",
      "locals",
    ]
  }

  required_fields {
    include {
      expose = false
    }
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }
}

