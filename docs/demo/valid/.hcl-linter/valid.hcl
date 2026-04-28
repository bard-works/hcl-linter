rules {
  block_order {
    enabled = true
    order = [
      "dependency",
      "include",
      "inputs",
      "locals",
      "terraform",
    ]
  }
}

