# List every environment in the hub.
data "cycle_environments" "all" {}

output "environment_names" {
  value = [for env in data.cycle_environments.all.environments : env.name]
}

output "live_environment_ids" {
  value = [
    for env in data.cycle_environments.all.environments : env.id
    if env.state == "live"
  ]
}
