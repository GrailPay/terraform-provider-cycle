resource "cycle_hub_role" "ci" {
  name       = "CI Deploy"
  identifier = "ci-deploy"
  rank       = 2

  capabilities = {
    specific = [
      "environments-view",
      "containers-view",
    ]
  }
}

# The secret is returned only at create time and is marked sensitive.
# Permissions come from role_id; there is no capabilities field on the key.
resource "cycle_api_key" "ci" {
  name    = "github-actions"
  role_id = cycle_hub_role.ci.id
  ips     = ["203.0.113.10"]
}
