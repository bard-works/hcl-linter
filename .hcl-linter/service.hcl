rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }

  block_order {
    enabled = true
    order   = ["locals"]
  }

  array_format {
    enabled             = true
    multiline_threshold = 2
  }
}
