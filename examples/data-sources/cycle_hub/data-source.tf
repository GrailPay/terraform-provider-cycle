# Look up the hub the provider is configured for.
data "cycle_hub" "current" {}

output "hub_name" {
  value = data.cycle_hub.current.name
}
