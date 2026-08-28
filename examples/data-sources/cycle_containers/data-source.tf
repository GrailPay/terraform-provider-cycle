# List every container in the hub.
data "cycle_containers" "all" {}

# Or only those in a specific environment.
data "cycle_containers" "api" {
  environment_id = "651efd54c53f7b6e2c5a9f21"
}

output "container_names" {
  value = [for c in data.cycle_containers.all.containers : c.name]
}

output "running_container_ids" {
  value = [
    for c in data.cycle_containers.api.containers : c.id
    if c.state == "running"
  ]
}
