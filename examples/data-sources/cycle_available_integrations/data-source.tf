# List vendors the hub can enable, flattened across catalog categories.
data "cycle_available_integrations" "all" {}

output "usable_vendors" {
  value = [
    for i in data.cycle_available_integrations.all.integrations : i.vendor
    if i.usable && !i.deprecated
  ]
}

output "infrastructure_providers" {
  value = [
    for i in data.cycle_available_integrations.all.integrations : i.vendor
    if i.category == "infrastructure-provider"
  ]
}
