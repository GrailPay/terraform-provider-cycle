# List locations advertised by a provider integration.
data "cycle_provider_locations" "equinix" {
  integration_id = var.provider_integration_id
}

output "compatible_location_ids" {
  value = [
    for loc in data.cycle_provider_locations.equinix.locations : loc.id
    if loc.compatible
  ]
}
