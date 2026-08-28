# Look up an SDN network by ID.
data "cycle_network" "shared" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "network_state" {
  value = data.cycle_network.shared.state
}
