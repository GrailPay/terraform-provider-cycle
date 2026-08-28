# The VPN is an environment singleton. This resource reconfigures the
# service that already exists on the environment.
resource "cycle_vpn" "api" {
  environment_id    = cycle_environment.api.id
  enable            = true
  high_availability = false
  auto_update       = true
  allow_internet    = false
  cycle_accounts    = true
  vpn_accounts      = true
}
