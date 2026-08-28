resource "cycle_hub_role" "developer" {
  name       = "Developer"
  identifier = "developer"
  rank       = 3

  capabilities = {
    specific = [
      "environments-view",
      "containers-view",
      "containers-manage",
    ]
  }
}

# Manage the role of an existing hub member. The account must already be a
# member of the hub (i.e. have accepted an invite); Cycle has no API to add
# members directly. Destroying this resource removes the member from the hub.
resource "cycle_hub_member" "teammate" {
  account_id = "651f1e95c3f1b2a4d8e7a789"
  role_id    = cycle_hub_role.developer.id
}
