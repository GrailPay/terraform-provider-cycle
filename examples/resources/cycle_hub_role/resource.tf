# A custom role granting read-only access to environments and containers.
resource "cycle_hub_role" "readonly" {
  name       = "Read Only"
  identifier = "read-only"
  rank       = 1

  capabilities = {
    specific = [
      "environments-view",
      "containers-view",
    ]
  }
}

# An admin-style role with every platform capability.
resource "cycle_hub_role" "admin" {
  name       = "Administrator"
  identifier = "administrator"
  rank       = 8

  capabilities = {
    all = true
  }
}
