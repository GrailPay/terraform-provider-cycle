# The gateway is an environment singleton. This resource reconfigures the
# service that already exists on the environment.
# Destroy only drops Terraform state; the Cycle gateway service remains.
resource "cycle_gateway_service" "api" {
  environment_id = cycle_environment.api.id
  auto_update    = true

  config = jsonencode({
    ipv4        = true
    ipv6        = false
    performance = false
  })
}
