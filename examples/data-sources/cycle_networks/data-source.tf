# List every SDN network in the hub.
data "cycle_networks" "all" {}

output "network_names" {
  value = [for n in data.cycle_networks.all.networks : n.name]
}

output "live_network_ids" {
  value = [
    for n in data.cycle_networks.all.networks : n.id
    if n.state == "live"
  ]
}
