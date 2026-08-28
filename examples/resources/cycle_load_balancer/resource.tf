# The load balancer is an environment singleton. This resource reconfigures
# the service that already exists on the environment.
resource "cycle_load_balancer" "api" {
  environment_id    = cycle_environment.api.id
  high_availability = false
  auto_update       = true

  # Discriminated union: type is "default", "haproxy", or "v1".
  config = jsonencode({
    type        = "default"
    ipv4        = true
    ipv6        = false
    performance = false
  })
}
