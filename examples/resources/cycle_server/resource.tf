# Look up a location and model from a provider integration, then provision
# one server into an existing cluster.
data "cycle_provider_locations" "equinix" {
  integration_id = var.provider_integration_id
}

data "cycle_provider_server_models" "equinix" {
  integration_id = var.provider_integration_id
}

resource "cycle_server" "worker" {
  cluster        = "production"
  integration_id = var.provider_integration_id
  location_id    = data.cycle_provider_locations.equinix.locations[0].id
  model_id       = data.cycle_provider_server_models.equinix.models[0].id

  nickname     = "worker-1"
  force_delete = false

  tags = ["workers"]
}
