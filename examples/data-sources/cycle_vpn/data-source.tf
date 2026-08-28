data "cycle_vpn" "api" {
  environment_id = cycle_environment.api.id
}

output "vpn_url" {
  value = data.cycle_vpn.api.url
}
