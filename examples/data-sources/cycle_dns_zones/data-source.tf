data "cycle_dns_zones" "all" {}

output "hosted_origins" {
  value = [
    for zone in data.cycle_dns_zones.all.zones : zone.origin
    if zone.hosted
  ]
}
