# builtin:terraform - recommended rules for Terraform (.tf) files.
# Formatting plus checks that catch mistakes terraform validate reports late
# or not at all: duplicate definitions and count/for_each misuse.
rules {
  array_format {
    enabled             = true
    multiline_threshold = 2
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }

  duplicates {
    enabled = true
    blocks  = ["resource", "data", "module", "variable", "output"]
  }

  count_for_each {
    enabled                = true
    warn_on_count_zero     = true
    warn_on_empty_for_each = true
    warn_on_conflict       = true
  }
}
