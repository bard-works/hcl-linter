rules {
  name_validation {
    enabled = true
    pattern = "^[a-z][a-z0-9_]*$"
    blocks = [
      "dependency",
      "include",
    ]
  }
}

