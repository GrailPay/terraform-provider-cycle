# List server models advertised by a provider integration.
data "cycle_provider_server_models" "equinix" {
  integration_id = var.provider_integration_id
}

output "compatible_model_ids" {
  value = [
    for model in data.cycle_provider_server_models.equinix.models : model.id
    if model.compatible
  ]
}
