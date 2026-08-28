# List every API key in the hub. Secrets are not included.
data "cycle_api_keys" "all" {}

output "api_key_names" {
  value = [for k in data.cycle_api_keys.all.api_keys : k.name]
}

output "live_api_key_ids" {
  value = [
    for k in data.cycle_api_keys.all.api_keys : k.id
    if k.state == "live"
  ]
}
