# List every integration in the hub.
data "cycle_integrations" "all" {}

output "integration_names" {
  value = [for i in data.cycle_integrations.all.integrations : i.name]
}

output "live_integration_ids" {
  value = [
    for i in data.cycle_integrations.all.integrations : i.id
    if i.state == "live"
  ]
}
