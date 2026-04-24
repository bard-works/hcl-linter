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

  key_value {
    enabled      = true
    key_case    = "snake_case"

    value_pattern = {
      region     = "^us-[a-z]+-[0-9]+$"
      account_id = "^[0-9]{12}$"
    }

    disallowed = ["secret", "password", "api_key", "token"]
  }
}