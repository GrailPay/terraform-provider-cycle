# Look up an external volume by ID.
data "cycle_external_volume" "data" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "volume_state" {
  value = data.cycle_external_volume.data.state
}
