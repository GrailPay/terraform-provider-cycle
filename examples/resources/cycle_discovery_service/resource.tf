# Discovery is an environment singleton. This resource reconfigures the
# service that already exists on the environment.
# Destroy only drops Terraform state; the Cycle discovery service remains.
resource "cycle_discovery_service" "api" {
  environment_id    = cycle_environment.api.id
  high_availability = false
  auto_update       = true

  config = jsonencode({
    dual_stack_legacy = true
  })
}
