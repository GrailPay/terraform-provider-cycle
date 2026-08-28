data "cycle_vpn_users" "api" {
  environment_id = cycle_environment.api.id
}

output "vpn_usernames" {
  value = [for user in data.cycle_vpn_users.api.users : user.username]
}
