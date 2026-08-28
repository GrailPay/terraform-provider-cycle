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

# Invite a teammate to the hub with the developer role. The invite is
# revoked if the resource is destroyed before it is accepted. Once accepted,
# the invite disappears and the membership can be managed with
# cycle_hub_member.
resource "cycle_hub_invite" "teammate" {
  recipient = "teammate@example.com"
  role_id   = cycle_hub_role.developer.id
}
