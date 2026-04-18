rules {
  blank_lines {
    enabled        = true
    within_blocks = true
  }

  block_order {
    enabled = true
    order   = ["locals"]
  }

  array_format {
    enabled            = true
    multiline_threshold = 2
  }

  count_for_each {
    enabled             = true
    warn_on_count_zero  = true
    warn_on_empty_for_each = true
    warn_on_conflict    = true
  }
}
